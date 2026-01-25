// internal/app/features/status/handler.go
package status

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/certcheck"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/config"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/dalemusser/waffle/server"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

var startTime = time.Now()

// Handler holds dependencies for the status page.
type Handler struct {
	Client  *mongo.Client
	BaseURL string
	Log     *zap.Logger
	CoreCfg *config.CoreConfig
	AppCfg  AppConfig
}

// AppConfig mirrors bootstrap.AppConfig for status display.
type AppConfig struct {
	// MongoDB
	MongoURI         string
	MongoDatabase    string
	MongoMaxPoolSize uint64
	MongoMinPoolSize uint64

	// Session
	SessionKey        string
	SessionName       string
	SessionDomain     string
	SessionMaxAge     time.Duration
	IdleLogoutEnabled bool
	IdleLogoutTimeout time.Duration
	IdleLogoutWarning time.Duration
	CSRFKey           string

	// Rate Limiting
	RateLimitEnabled       bool
	RateLimitLoginAttempts int
	RateLimitLoginWindow   time.Duration
	RateLimitLoginLockout  time.Duration

	// API
	APIKey string

	// Storage
	StorageType        string
	StorageLocalPath   string
	StorageLocalURL    string
	StorageS3Region    string
	StorageS3Bucket    string
	StorageS3Prefix    string
	StorageCFURL       string
	StorageCFKeyPairID string
	StorageCFKeyPath   string

	// Email/SMTP
	MailSMTPHost      string
	MailSMTPPort      int
	MailSMTPUser      string
	MailSMTPPass      string
	MailFrom          string
	MailFromName      string
	BaseURL           string
	EmailVerifyExpiry time.Duration

	// Audit
	AuditLogAuth  string
	AuditLogAdmin string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string

	// Admin seeding
	SeedAdminEmail string
	SeedAdminName  string
}

// NewHandler creates a new status Handler.
func NewHandler(client *mongo.Client, baseURL string, coreCfg *config.CoreConfig, appCfg AppConfig, logger *zap.Logger) *Handler {
	return &Handler{
		Client:  client,
		BaseURL: baseURL,
		CoreCfg: coreCfg,
		AppCfg:  appCfg,
		Log:     logger,
	}
}

// ConfigItem represents a single configuration variable for display.
type ConfigItem struct {
	Name  string
	Value string
}

// ConfigGroup represents a logical group of configuration items.
type ConfigGroup struct {
	Name  string
	Items []ConfigItem
}

// statusVM is the view model for the status page.
type statusVM struct {
	viewdata.BaseVM

	// Certificate info
	CertHost          string
	CertExpiresAt     string
	CertExpiresIn     string // detailed countdown: "82 days, 14 hours, 23 minutes"
	CertDaysLeft      int
	CertIssuer        string
	CertValid         bool
	CertError         string
	CertWarning       bool   // true if expiring within 14 days
	CanRenewCert      bool   // true if Let's Encrypt is being used
	CertChallengeType string // "http-01" or "dns-01"
	RenewSuccess      bool   // true if renewal just succeeded
	RenewError        string // error message if renewal failed

	// Database info
	DBConnected bool
	DBError     string
	DBPingMS    int64  // ping latency in milliseconds
	DBVersion   string // MongoDB server version

	// System info
	GoVersion    string
	Uptime       string
	NumGoroutine int
	MemAlloc     string

	// Configuration (organized by groups)
	ConfigGroups []ConfigGroup
}

// Serve handles GET /admin/status.
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	db := h.Client.Database(h.AppCfg.MongoDatabase)
	vm := statusVM{
		BaseVM:       viewdata.NewBaseVM(r, db, "System Status", "/dashboard"),
		GoVersion:    runtime.Version(),
		Uptime:       formatDuration(time.Since(startTime)),
		NumGoroutine: runtime.NumGoroutine(),
	}

	// Check for renewal success message
	if r.URL.Query().Get("renewed") == "1" {
		vm.RenewSuccess = true
	}

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	vm.MemAlloc = formatBytes(m.Alloc)

	// Check database with ping latency
	pingStart := time.Now()
	if err := h.Client.Ping(ctx, readpref.Primary()); err != nil {
		vm.DBConnected = false
		vm.DBError = err.Error()
		h.Log.Warn("status page: database ping failed", zap.Error(err))
	} else {
		vm.DBConnected = true
		vm.DBPingMS = time.Since(pingStart).Milliseconds()

		// Get server version
		var result bson.M
		if err := h.Client.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&result); err == nil {
			if version, ok := result["version"].(string); ok {
				vm.DBVersion = version
			}
		}
	}

	// Check certificate
	if h.BaseURL != "" {
		certInfo := certcheck.Check(h.BaseURL)
		vm.CertHost = certInfo.Host
		vm.CertValid = certInfo.IsValid
		vm.CertError = certInfo.Error
		vm.CertDaysLeft = certInfo.DaysLeft
		vm.CertIssuer = certInfo.Issuer
		if !certInfo.ExpiresAt.IsZero() {
			vm.CertExpiresAt = certInfo.ExpiresAt.Format("Jan 02, 2006 15:04 MST")
			vm.CertExpiresIn = formatExpiresIn(time.Until(certInfo.ExpiresAt))
		}
		vm.CertWarning = certInfo.DaysLeft > 0 && certInfo.DaysLeft <= 14
	}

	// Check if certificate renewal is available (Let's Encrypt)
	if renewer := server.GetCertRenewer(); renewer != nil {
		vm.CanRenewCert = true
		vm.CertChallengeType = renewer.ChallengeType()
	}

	// Build configuration groups
	vm.ConfigGroups = h.buildConfigGroups()

	templates.Render(w, r, "admin_status", vm)
}

// HandleRenew handles POST /admin/status/renew to force certificate renewal.
func (h *Handler) HandleRenew(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	renewer := server.GetCertRenewer()
	if renewer == nil {
		http.Error(w, "Certificate renewal not available", http.StatusBadRequest)
		return
	}

	h.Log.Info("forcing certificate renewal",
		zap.String("challenge_type", renewer.ChallengeType()))

	newExpiry, err := renewer.ForceRenewal(ctx)
	if err != nil {
		h.Log.Error("certificate renewal failed", zap.Error(err))
		http.Error(w, "Renewal failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.Log.Info("certificate renewal succeeded",
		zap.Time("new_expiry", newExpiry))

	// Redirect back to status page with success message
	http.Redirect(w, r, "/admin/status?renewed=1", http.StatusSeeOther)
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return formatPlural(days, "day") + " " + formatPlural(hours, "hour")
	}
	if hours > 0 {
		return formatPlural(hours, "hour") + " " + formatPlural(minutes, "min")
	}
	return formatPlural(minutes, "min")
}

// formatExpiresIn formats time until expiration with days, hours, and minutes.
func formatExpiresIn(d time.Duration) string {
	if d < 0 {
		return "expired"
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	return formatPlural(days, "day") + ", " + formatPlural(hours, "hour") + ", " + formatPlural(minutes, "minute")
}

func formatPlural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return formatUint(uint64(n)) + " " + unit + "s"
}

// formatBytes formats bytes in a human-readable way.
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return formatUint(b) + " B"
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return formatFloat(float64(b)/float64(div)) + " " + string("KMGTPE"[exp]) + "iB"
}

func formatUint(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func formatFloat(f float64) string {
	// Simple formatting to 1 decimal place
	i := int(f * 10)
	return formatUint(uint64(i/10)) + "." + string(rune('0'+i%10))
}

// buildConfigGroups creates organized groups of config items for display.
func (h *Handler) buildConfigGroups() []ConfigGroup {
	groups := []ConfigGroup{}

	// Helper to mask sensitive values
	mask := func(s string) string {
		if s == "" {
			return ""
		}
		if len(s) <= 4 {
			return "****"
		}
		return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
	}

	// Helper to format string slices
	join := func(s []string) string {
		if len(s) == 0 {
			return ""
		}
		return strings.Join(s, ", ")
	}

	// Helper to format bool
	boolStr := func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	}

	// Environment
	if h.CoreCfg != nil {
		groups = append(groups, ConfigGroup{
			Name: "Environment",
			Items: []ConfigItem{
				{Name: "env", Value: h.CoreCfg.Env},
				{Name: "log_level", Value: h.CoreCfg.LogLevel},
			},
		})
	}

	// HTTP Server
	if h.CoreCfg != nil {
		groups = append(groups, ConfigGroup{
			Name: "HTTP Server",
			Items: []ConfigItem{
				{Name: "http_port", Value: fmt.Sprintf("%d", h.CoreCfg.HTTP.HTTPPort)},
				{Name: "https_port", Value: fmt.Sprintf("%d", h.CoreCfg.HTTP.HTTPSPort)},
				{Name: "use_https", Value: boolStr(h.CoreCfg.HTTP.UseHTTPS)},
				{Name: "read_timeout", Value: h.CoreCfg.HTTP.ReadTimeout.String()},
				{Name: "read_header_timeout", Value: h.CoreCfg.HTTP.ReadHeaderTimeout.String()},
				{Name: "write_timeout", Value: h.CoreCfg.HTTP.WriteTimeout.String()},
				{Name: "idle_timeout", Value: h.CoreCfg.HTTP.IdleTimeout.String()},
				{Name: "shutdown_timeout", Value: h.CoreCfg.HTTP.ShutdownTimeout.String()},
				{Name: "max_request_body_bytes", Value: fmt.Sprintf("%d", h.CoreCfg.MaxRequestBodyBytes)},
				{Name: "enable_compression", Value: boolStr(h.CoreCfg.EnableCompression)},
				{Name: "compression_level", Value: fmt.Sprintf("%d", h.CoreCfg.CompressionLevel)},
			},
		})
	}

	// TLS/SSL
	if h.CoreCfg != nil {
		groups = append(groups, ConfigGroup{
			Name: "TLS/SSL",
			Items: []ConfigItem{
				{Name: "cert_file", Value: h.CoreCfg.TLS.CertFile},
				{Name: "key_file", Value: h.CoreCfg.TLS.KeyFile},
				{Name: "use_lets_encrypt", Value: boolStr(h.CoreCfg.TLS.UseLetsEncrypt)},
				{Name: "lets_encrypt_email", Value: h.CoreCfg.TLS.LetsEncryptEmail},
				{Name: "lets_encrypt_cache_dir", Value: h.CoreCfg.TLS.LetsEncryptCacheDir},
				{Name: "lets_encrypt_challenge", Value: h.CoreCfg.TLS.LetsEncryptChallenge},
				{Name: "domain", Value: h.CoreCfg.TLS.Domain},
				{Name: "domains", Value: join(h.CoreCfg.TLS.Domains)},
				{Name: "route53_hosted_zone_id", Value: h.CoreCfg.TLS.Route53HostedZoneID},
				{Name: "acme_directory_url", Value: h.CoreCfg.TLS.ACMEDirectoryURL},
			},
		})
	}

	// CORS
	if h.CoreCfg != nil {
		groups = append(groups, ConfigGroup{
			Name: "CORS",
			Items: []ConfigItem{
				{Name: "enable_cors", Value: boolStr(h.CoreCfg.CORS.EnableCORS)},
				{Name: "cors_allowed_origins", Value: join(h.CoreCfg.CORS.CORSAllowedOrigins)},
				{Name: "cors_allowed_methods", Value: join(h.CoreCfg.CORS.CORSAllowedMethods)},
				{Name: "cors_allowed_headers", Value: join(h.CoreCfg.CORS.CORSAllowedHeaders)},
				{Name: "cors_exposed_headers", Value: join(h.CoreCfg.CORS.CORSExposedHeaders)},
				{Name: "cors_allow_credentials", Value: boolStr(h.CoreCfg.CORS.CORSAllowCredentials)},
				{Name: "cors_max_age", Value: fmt.Sprintf("%d", h.CoreCfg.CORS.CORSMaxAge)},
			},
		})
	}

	// Database
	dbItems := []ConfigItem{
		{Name: "mongo_uri", Value: mask(h.AppCfg.MongoURI)},
		{Name: "mongo_database", Value: h.AppCfg.MongoDatabase},
		{Name: "mongo_max_pool_size", Value: fmt.Sprintf("%d", h.AppCfg.MongoMaxPoolSize)},
		{Name: "mongo_min_pool_size", Value: fmt.Sprintf("%d", h.AppCfg.MongoMinPoolSize)},
	}
	if h.CoreCfg != nil {
		dbItems = append(dbItems,
			ConfigItem{Name: "db_connect_timeout", Value: h.CoreCfg.DBConnectTimeout.String()},
			ConfigItem{Name: "index_boot_timeout", Value: h.CoreCfg.IndexBootTimeout.String()},
		)
	}
	groups = append(groups, ConfigGroup{Name: "Database", Items: dbItems})

	// Session & Security
	groups = append(groups, ConfigGroup{
		Name: "Session & Security",
		Items: []ConfigItem{
			{Name: "session_key", Value: mask(h.AppCfg.SessionKey)},
			{Name: "session_name", Value: h.AppCfg.SessionName},
			{Name: "session_domain", Value: h.AppCfg.SessionDomain},
			{Name: "session_max_age", Value: h.AppCfg.SessionMaxAge.String()},
			{Name: "idle_logout_enabled", Value: boolStr(h.AppCfg.IdleLogoutEnabled)},
			{Name: "idle_logout_timeout", Value: h.AppCfg.IdleLogoutTimeout.String()},
			{Name: "idle_logout_warning", Value: h.AppCfg.IdleLogoutWarning.String()},
			{Name: "rate_limit_enabled", Value: boolStr(h.AppCfg.RateLimitEnabled)},
			{Name: "rate_limit_login_attempts", Value: fmt.Sprintf("%d", h.AppCfg.RateLimitLoginAttempts)},
			{Name: "rate_limit_login_window", Value: h.AppCfg.RateLimitLoginWindow.String()},
			{Name: "rate_limit_login_lockout", Value: h.AppCfg.RateLimitLoginLockout.String()},
			{Name: "csrf_key", Value: mask(h.AppCfg.CSRFKey)},
			{Name: "api_key", Value: mask(h.AppCfg.APIKey)},
		},
	})

	// Storage
	groups = append(groups, ConfigGroup{
		Name: "Storage",
		Items: []ConfigItem{
			{Name: "storage_type", Value: h.AppCfg.StorageType},
			{Name: "storage_local_path", Value: h.AppCfg.StorageLocalPath},
			{Name: "storage_local_url", Value: h.AppCfg.StorageLocalURL},
			{Name: "storage_s3_region", Value: h.AppCfg.StorageS3Region},
			{Name: "storage_s3_bucket", Value: h.AppCfg.StorageS3Bucket},
			{Name: "storage_s3_prefix", Value: h.AppCfg.StorageS3Prefix},
			{Name: "storage_cf_url", Value: h.AppCfg.StorageCFURL},
			{Name: "storage_cf_keypair_id", Value: h.AppCfg.StorageCFKeyPairID},
			{Name: "storage_cf_key_path", Value: h.AppCfg.StorageCFKeyPath},
		},
	})

	// Email/SMTP
	groups = append(groups, ConfigGroup{
		Name: "Email/SMTP",
		Items: []ConfigItem{
			{Name: "mail_smtp_host", Value: h.AppCfg.MailSMTPHost},
			{Name: "mail_smtp_port", Value: fmt.Sprintf("%d", h.AppCfg.MailSMTPPort)},
			{Name: "mail_smtp_user", Value: h.AppCfg.MailSMTPUser},
			{Name: "mail_smtp_pass", Value: mask(h.AppCfg.MailSMTPPass)},
			{Name: "mail_from", Value: h.AppCfg.MailFrom},
			{Name: "mail_from_name", Value: h.AppCfg.MailFromName},
			{Name: "base_url", Value: h.AppCfg.BaseURL},
			{Name: "email_verify_expiry", Value: h.AppCfg.EmailVerifyExpiry.String()},
		},
	})

	// Authentication
	groups = append(groups, ConfigGroup{
		Name: "Authentication",
		Items: []ConfigItem{
			{Name: "google_client_id", Value: mask(h.AppCfg.GoogleClientID)},
			{Name: "google_client_secret", Value: mask(h.AppCfg.GoogleClientSecret)},
		},
	})

	// Audit Logging
	groups = append(groups, ConfigGroup{
		Name: "Audit Logging",
		Items: []ConfigItem{
			{Name: "audit_log_auth", Value: h.AppCfg.AuditLogAuth},
			{Name: "audit_log_admin", Value: h.AppCfg.AuditLogAdmin},
		},
	})

	// Admin Seeding
	groups = append(groups, ConfigGroup{
		Name: "Admin Seeding",
		Items: []ConfigItem{
			{Name: "seed_admin_email", Value: h.AppCfg.SeedAdminEmail},
			{Name: "seed_admin_name", Value: h.AppCfg.SeedAdminName},
		},
	})

	return groups
}

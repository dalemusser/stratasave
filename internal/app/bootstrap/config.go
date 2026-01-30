// internal/app/bootstrap/config.go
package bootstrap

import (
	"fmt"
	"time"

	"github.com/dalemusser/waffle/config"
	wafflemongo "github.com/dalemusser/waffle/pantry/mongo"
	"go.uber.org/zap"
)

// EnvVarPrefix is the prefix for environment variables.
// Change this constant when forking stratasave for a new project.
// For example, change to "STRATALOG" for a stratalog project.
const EnvVarPrefix = "STRATASAVE"

// appConfigKeys defines the configuration keys for this application.
// These are loaded via WAFFLE's config system with support for:
//   - Config files: mongo_uri, session_name, etc.
//   - Environment variables: STRATASAVE_MONGO_URI, STRATASAVE_SESSION_NAME, etc.
//   - Command-line flags: --mongo_uri, --session_name, etc.
var appConfigKeys = []config.AppKey{
	{Name: "mongo_uri", Default: "mongodb://localhost:27017", Desc: "MongoDB connection URI"},
	{Name: "mongo_database", Default: "stratasave", Desc: "MongoDB database name"},
	{Name: "mongo_max_pool_size", Default: 100, Desc: "MongoDB max connection pool size (default: 100)"},
	{Name: "mongo_min_pool_size", Default: 10, Desc: "MongoDB min connection pool size (default: 10)"},
	{Name: "session_key", Default: "dev-only-change-me-please-0123456789ABCDEF", Desc: "Session signing key (must be strong in production)"},
	{Name: "session_name", Default: "stratasave-session", Desc: "Session cookie name"},
	{Name: "session_domain", Default: "", Desc: "Session cookie domain (blank means current host)"},
	{Name: "session_max_age", Default: "24h", Desc: "Session cookie max age (e.g., 24h, 720h, 30m)"},

	// Idle logout configuration
	{Name: "idle_logout_enabled", Default: false, Desc: "Enable automatic logout after idle time"},
	{Name: "idle_logout_timeout", Default: "30m", Desc: "Idle timeout duration before logout"},
	{Name: "idle_logout_warning", Default: "5m", Desc: "Warning time before idle logout"},

	// Rate limiting configuration
	{Name: "rate_limit_enabled", Default: true, Desc: "Enable rate limiting for login attempts"},
	{Name: "rate_limit_login_attempts", Default: 5, Desc: "Max failed login attempts before lockout"},
	{Name: "rate_limit_login_window", Default: "15m", Desc: "Time window for counting failed attempts"},
	{Name: "rate_limit_login_lockout", Default: "15m", Desc: "Lockout duration after exceeding limit"},

	{Name: "csrf_key", Default: "dev-only-csrf-key-please-change-0123456789", Desc: "CSRF token signing key (32+ chars in production)"},

	// API key configuration (for external API consumers using Bearer token auth)
	{Name: "api_key", Default: "", Desc: "API key for external API access (leave empty to disable API key auth)"},

	// File storage configuration
	{Name: "storage_type", Default: "local", Desc: "Storage backend: 'local' or 's3'"},
	{Name: "storage_local_path", Default: "./uploads", Desc: "Local storage path for uploaded files"},
	{Name: "storage_local_url", Default: "/files", Desc: "URL prefix for serving local files"},

	// S3/CloudFront configuration
	{Name: "storage_s3_region", Default: "", Desc: "AWS region for S3"},
	{Name: "storage_s3_bucket", Default: "", Desc: "S3 bucket name"},
	{Name: "storage_s3_prefix", Default: "uploads/", Desc: "S3 key prefix"},
	{Name: "storage_cf_url", Default: "", Desc: "CloudFront distribution URL"},
	{Name: "storage_cf_keypair_id", Default: "", Desc: "CloudFront key pair ID"},
	{Name: "storage_cf_key_path", Default: "", Desc: "Path to CloudFront private key file"},

	// Email/SMTP configuration
	{Name: "mail_smtp_host", Default: "localhost", Desc: "SMTP server host"},
	{Name: "mail_smtp_port", Default: 1025, Desc: "SMTP server port"},
	{Name: "mail_smtp_user", Default: "", Desc: "SMTP username"},
	{Name: "mail_smtp_pass", Default: "", Desc: "SMTP password"},
	{Name: "mail_from", Default: "noreply@example.com", Desc: "From email address"},
	{Name: "mail_from_name", Default: "StrataSave", Desc: "From display name"},

	// Base URL for email links (magic links, etc.)
	{Name: "base_url", Default: "http://localhost:8080", Desc: "Base URL for email links"},

	// Email verification settings
	{Name: "email_verify_expiry", Default: "10m", Desc: "Email verification code/link expiry (e.g., 10m, 1h, 90s)"},

	// Audit logging settings
	{Name: "audit_log_auth", Default: "all", Desc: "Auth event logging: 'all' (db+log), 'db', 'log', or 'off'"},
	{Name: "audit_log_admin", Default: "all", Desc: "Admin event logging: 'all' (db+log), 'db', 'log', or 'off'"},

	// Google OAuth configuration
	{Name: "google_client_id", Default: "", Desc: "Google OAuth2 client ID"},
	{Name: "google_client_secret", Default: "", Desc: "Google OAuth2 client secret"},

	// Admin seeding configuration
	{Name: "seed_admin_email", Default: "", Desc: "Email of admin user to create on startup"},
	{Name: "seed_admin_name", Default: "Admin", Desc: "Name of admin user to create on startup"},

	// Save retention configuration
	{Name: "max_saves_per_user", Default: "5", Desc: "Max saves per user per game ('all' or a number)"},

	// API stats configuration
	{Name: "api_stats_bucket", Default: "1h", Desc: "API stats bucket duration (e.g., '1m', '15m', '1h', '24h')"},
}

// LoadConfig loads WAFFLE core config and app-specific config.
//
// It is called early in startup so that both WAFFLE and the app have
// access to configuration before any backends or handlers are built.
// CoreConfig comes from the shared WAFFLE layer; AppConfig is specific
// to this app and can be extended as the app grows.
//
// WAFFLE's config.LoadWithAppConfig handles:
//   - Loading from .env files
//   - Loading from config.yaml/json/toml files
//   - Reading environment variables (WAFFLE_* for core, STRATASAVE_* for app)
//   - Parsing command-line flags
//   - Merging with precedence: flags > env > files > defaults
func LoadConfig(logger *zap.Logger) (*config.CoreConfig, AppConfig, error) {
	coreCfg, appValues, err := config.LoadWithAppConfig(logger, EnvVarPrefix, appConfigKeys)
	if err != nil {
		return nil, AppConfig{}, err
	}

	appCfg := AppConfig{
		MongoURI:         appValues.String("mongo_uri"),
		MongoDatabase:    appValues.String("mongo_database"),
		MongoMaxPoolSize: uint64(appValues.Int("mongo_max_pool_size")),
		MongoMinPoolSize: uint64(appValues.Int("mongo_min_pool_size")),
		SessionKey:       appValues.String("session_key"),
		SessionName:      appValues.String("session_name"),
		SessionDomain:    appValues.String("session_domain"),
		SessionMaxAge:    appValues.Duration("session_max_age", 24*time.Hour),

		// Idle logout
		IdleLogoutEnabled: appValues.Bool("idle_logout_enabled"),
		IdleLogoutTimeout: appValues.Duration("idle_logout_timeout", 30*time.Minute),
		IdleLogoutWarning: appValues.Duration("idle_logout_warning", 5*time.Minute),

		// Rate limiting
		RateLimitEnabled:       appValues.Bool("rate_limit_enabled"),
		RateLimitLoginAttempts: appValues.Int("rate_limit_login_attempts"),
		RateLimitLoginWindow:   appValues.Duration("rate_limit_login_window", 15*time.Minute),
		RateLimitLoginLockout:  appValues.Duration("rate_limit_login_lockout", 15*time.Minute),

		CSRFKey: appValues.String("csrf_key"),
		APIKey:           appValues.String("api_key"),

		// File storage
		StorageType:      appValues.String("storage_type"),
		StorageLocalPath: appValues.String("storage_local_path"),
		StorageLocalURL:  appValues.String("storage_local_url"),

		// S3/CloudFront
		StorageS3Region:    appValues.String("storage_s3_region"),
		StorageS3Bucket:    appValues.String("storage_s3_bucket"),
		StorageS3Prefix:    appValues.String("storage_s3_prefix"),
		StorageCFURL:       appValues.String("storage_cf_url"),
		StorageCFKeyPairID: appValues.String("storage_cf_keypair_id"),
		StorageCFKeyPath:   appValues.String("storage_cf_key_path"),

		// Email/SMTP
		MailSMTPHost: appValues.String("mail_smtp_host"),
		MailSMTPPort: appValues.Int("mail_smtp_port"),
		MailSMTPUser: appValues.String("mail_smtp_user"),
		MailSMTPPass: appValues.String("mail_smtp_pass"),
		MailFrom:     appValues.String("mail_from"),
		MailFromName: appValues.String("mail_from_name"),

		// Base URL
		BaseURL: appValues.String("base_url"),

		// Email verification
		EmailVerifyExpiry: appValues.Duration("email_verify_expiry", 10*time.Minute),

		// Audit logging
		AuditLogAuth:  appValues.String("audit_log_auth"),
		AuditLogAdmin: appValues.String("audit_log_admin"),

		// Google OAuth
		GoogleClientID:     appValues.String("google_client_id"),
		GoogleClientSecret: appValues.String("google_client_secret"),

		// Admin seeding
		SeedAdminEmail: appValues.String("seed_admin_email"),
		SeedAdminName:  appValues.String("seed_admin_name"),

		// Save retention
		MaxSavesPerUser: appValues.String("max_saves_per_user"),

		// API stats
		APIStatsBucket: appValues.Duration("api_stats_bucket", 1*time.Hour),
	}

	return coreCfg, appCfg, nil
}

// ValidateConfig performs app-specific config validation.
//
// Return nil to accept the loaded config, or an error to abort startup.
// This is the right place to enforce required fields or invariants that
// involve both the core and app configs.
func ValidateConfig(coreCfg *config.CoreConfig, appCfg AppConfig, logger *zap.Logger) error {
	if err := wafflemongo.ValidateURI(appCfg.MongoURI); err != nil {
		logger.Error("invalid MongoDB URI", zap.Error(err))
		return fmt.Errorf("invalid MongoDB URI: %w", err)
	}

	return nil
}

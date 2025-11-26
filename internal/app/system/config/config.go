// internal/app/system/config/config.go
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds every parameter a running stratalog node needs.
type Config struct {
	/* runtime */
	Env      string `mapstructure:"env"`       // "dev" | "prod"
	LogLevel string `mapstructure:"log_level"` // debug, info, warn, error …

	/* network */
	HTTPPort  int  `mapstructure:"http_port"`
	HTTPSPort int  `mapstructure:"https_port"`
	UseHTTPS  bool `mapstructure:"use_https"`

	/* TLS / ACME */
	CertFile            string `mapstructure:"cert_file"`
	KeyFile             string `mapstructure:"key_file"`
	UseLetsEncrypt      bool   `mapstructure:"use_lets_encrypt"`
	LetsEncryptEmail    string `mapstructure:"lets_encrypt_email"`
	LetsEncryptCacheDir string `mapstructure:"lets_encrypt_cache_dir"`
	Domain              string `mapstructure:"domain"`

	// LetsEncryptChallenge selects which ACME challenge type to use when
	// UseLetsEncrypt is true. Supported values:
	//   - "http-01" (default; uses an HTTP challenge endpoint)
	//   - "dns-01"  (for use with Route 53 DNS TXT records)
	LetsEncryptChallenge string `mapstructure:"lets_encrypt_challenge"`

	// Route53HostedZoneID is required when using DNS-01 with Route 53 so the
	// ACME client knows which hosted zone to update.
	Route53HostedZoneID string `mapstructure:"route53_hosted_zone_id"`

	/* infrastructure */
	MongoURI      string `mapstructure:"mongo_uri"`
	MongoDatabase string `mapstructure:"mongo_database"`

	// IndexBootTimeout bounds the time allowed at startup to ensure indexes.
	// Example values: "90s", "2m", "120s".
	IndexBootTimeout time.Duration `mapstructure:"index_boot_timeout"`

	/* security */
	// IngestAPIKey protects the log ingestion endpoints (POST /logs, batches, etc.).
	IngestAPIKey string `mapstructure:"ingest_api_key"`
	// AdminAPIKey can protect dev views/downloads separately; if empty, you can
	// choose at the handler level whether to reuse IngestAPIKey or allow open.
	AdminAPIKey string `mapstructure:"admin_api_key"`

	/* misc */
	EnableCompression bool `mapstructure:"enable_compression"`
	EnableCORS        bool `mapstructure:"enable_cors"`

	/* CORS (configurable; JSON arrays for lists) */
	CORSAllowedOrigins   []string `mapstructure:"cors_allowed_origins"`
	CORSAllowedMethods   []string `mapstructure:"cors_allowed_methods"`
	CORSAllowedHeaders   []string `mapstructure:"cors_allowed_headers"`
	CORSExposedHeaders   []string `mapstructure:"cors_exposed_headers"`
	CORSAllowCredentials bool     `mapstructure:"cors_allow_credentials"`
	CORSMaxAge           int      `mapstructure:"cors_max_age"`
}

/*─────────────────────────────────────────────────────────────────────────────*
| Public helpers                                                               |
*─────────────────────────────────────────────────────────────────────────────*/

// Dump returns a pretty, redacted JSON string of the config for debugging.
// Never logs secrets; use at debug level only.
func (c Config) Dump() string {
	s := c.redactedCopy()
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}

/*─────────────────────────────────────────────────────────────────────────────*
| Config loader                                                                |
*─────────────────────────────────────────────────────────────────────────────*/

// Load merges defaults ⟶ config.* file(s) ⟶ env vars ⟶ explicit flags into one Config.
// Final precedence (highest wins): flags(explicit) > env > config > defaults.
func Load() (*Config, error) {
	/* ------------------------------------------------------------------ *
	| 0) Optionally load .env (safe: real env still wins over .env)       |
	* ------------------------------------------------------------------ */
	if err := godotenv.Load(); err == nil {
		zap.L().Info("Loaded .env file")
	}

	/* ------------------------------------------------------------------ *
	| 1) Define flags (only *explicitly set* flags will override)         |
	|    NOTE: list flags accept JSON arrays (e.g., '["GET","POST"]').    |
	* ------------------------------------------------------------------ */
	pflag.String("env", "dev", `Runtime environment "dev"|"prod"`)
	pflag.String("log_level", "debug", "Log level")

	pflag.Int("http_port", 80, "HTTP port")
	pflag.Int("https_port", 443, "HTTPS port")
	pflag.Bool("use_https", false, "Serve HTTPS")

	pflag.String("mongo_uri", "mongodb://localhost:27017", "Mongo URI")
	pflag.String("mongo_database", "stratalog", "Mongo database")

	pflag.String("index_boot_timeout", "120s", "Startup timeout for building DB indexes (e.g., \"90s\", \"2m\")")

	// TLS / Let’s Encrypt
	pflag.Bool("use_lets_encrypt", false, "Use Let's Encrypt")
	pflag.String("lets_encrypt_email", "", "ACME account e-mail")
	pflag.String("lets_encrypt_cache_dir", "letsencrypt-cache", "ACME cache dir")
	pflag.String("cert_file", "", "TLS cert file (manual TLS)")
	pflag.String("key_file", "", "TLS key file  (manual TLS)")
	pflag.String("domain", "", "Domain for TLS or ACME")
	pflag.String("lets_encrypt_challenge", "http-01", "ACME challenge type: http-01 or dns-01")
	pflag.String("route53_hosted_zone_id", "", "Route 53 hosted zone ID for DNS-01 challenge")

	// misc / compression + CORS
	pflag.Bool("enable_compression", true, "Enable HTTP compression")
	pflag.Bool("enable_cors", false, "Enable CORS") // default false (neutral)

	// CORS lists as JSON strings
	pflag.String("cors_allowed_origins", "", `JSON array of origins, e.g. '["https://a.example","https://b.example"]'`)
	pflag.String("cors_allowed_methods", "", `JSON array of methods, e.g. '["GET","POST"]'`)
	pflag.String("cors_allowed_headers", "", `JSON array of headers, e.g. '["Accept","Authorization"]'`)
	pflag.String("cors_exposed_headers", "", `JSON array of headers, e.g. '["Link"]'`)
	pflag.Bool("cors_allow_credentials", false, "CORS: allow credentials")
	pflag.Int("cors_max_age", 0, "CORS: max age seconds (0 disables cache)")

	// API keys
	pflag.String("ingest_api_key", "", "API key for log ingestion endpoints")
	pflag.String("admin_api_key", "", "API key for admin/view endpoints (optional)")

	pflag.Parse()

	/* ------------------------------------------------------------------ *
	| 2) Viper + env                                                      |
	* ------------------------------------------------------------------ */
	v := viper.New()
	v.SetEnvPrefix("STRATALOG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Bind env for all keys so Unmarshal sees them.
	for _, k := range allKeys() {
		_ = v.BindEnv(k)
	}

	/* ------------------------------------------------------------------ *
	| 3) Optional config.* files (yaml|yml|json|toml)                     |
	|    Log each merged file.                                            |
	* ------------------------------------------------------------------ */

	for _, ext := range [...]string{"yaml", "yml", "json", "toml"} {
		file := "config." + ext
		if _, err := os.Stat(file); err != nil {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			zap.L().Warn("cannot read config file", zap.String("file", file), zap.Error(err))
			continue
		}
		// Tell viper explicitly what we’re about to feed it.
		v.SetConfigType(ext)
		if err := v.MergeConfig(bytes.NewReader(b)); err != nil {
			zap.L().Warn("cannot decode config file", zap.String("file", file), zap.Error(err))
			continue
		}
		zap.L().Info("Loaded config file", zap.String("file", file))
	}

	/* ------------------------------------------------------------------ *
	| 4) Defaults (lowest precedence)                                     |
	|    Neutral defaults for CORS (no project-specific values).          |
	* ------------------------------------------------------------------ */
	setDefaults(v)

	/* ------------------------------------------------------------------ *
	| 5) Apply *explicit* flags (highest precedence)                      |
	|    Bind only flags the user actually set (so flag defaults don't    |
	|    mask env/config values).                                         |
	* ------------------------------------------------------------------ */
	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			_ = v.BindPFlag(f.Name, f)
		}
	})

	/* ------------------------------------------------------------------ *
	| 6) Normalize list keys (accept JSON strings → []string)             |
	|    Works for both env vars and flags that provided JSON strings.    |
	* ------------------------------------------------------------------ */
	if err := normalizeListKeys(v,
		"cors_allowed_origins",
		"cors_allowed_methods",
		"cors_allowed_headers",
		"cors_exposed_headers",
	); err != nil {
		return nil, err
	}

	/* ------------------------------------------------------------------ *
	| 7) Build struct                                                     |
	* ------------------------------------------------------------------ */
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}
	// Robustly parse index_boot_timeout (accepts "90s", "2m", or a bare number = seconds)
	dur, err := parseDurationFlexible(v.Get("index_boot_timeout"), 120*time.Second)
	if err != nil {
		zap.L().Warn("invalid index_boot_timeout; using default 120s",
			zap.Any("value", v.Get("index_boot_timeout")), zap.Error(err))
	}
	cfg.IndexBootTimeout = dur

	/* ------------------------------------------------------------------ *
	| 8) Validate (collect all errors)                                    |
	* ------------------------------------------------------------------ */
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

/*─────────────────────────────────────────────────────────────────────────────*
| Internals                                                                    |
*─────────────────────────────────────────────────────────────────────────────*/

func allKeys() []string {
	return []string{
		"env", "log_level",
		"http_port", "https_port", "use_https",
		"mongo_uri", "mongo_database",
		"index_boot_timeout",
		"use_lets_encrypt", "lets_encrypt_email", "lets_encrypt_cache_dir",
		"cert_file", "key_file", "domain",
		"lets_encrypt_challenge", "route53_hosted_zone_id",
		"enable_compression", "enable_cors",
		// CORS
		"cors_allowed_origins", "cors_allowed_methods", "cors_allowed_headers",
		"cors_exposed_headers", "cors_allow_credentials", "cors_max_age",
		// API keys
		"ingest_api_key", "admin_api_key",
	}
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("env", "dev")
	v.SetDefault("log_level", "debug")

	v.SetDefault("http_port", 80)
	v.SetDefault("https_port", 443)
	v.SetDefault("use_https", false)

	v.SetDefault("mongo_uri", "mongodb://localhost:27017")
	v.SetDefault("mongo_database", "stratalog")
	v.SetDefault("index_boot_timeout", "120s")

	v.SetDefault("use_lets_encrypt", false)
	v.SetDefault("lets_encrypt_email", "")
	v.SetDefault("lets_encrypt_cache_dir", "letsencrypt-cache")
	v.SetDefault("cert_file", "")
	v.SetDefault("key_file", "")
	v.SetDefault("domain", "")
	v.SetDefault("lets_encrypt_challenge", "http-01")
	v.SetDefault("route53_hosted_zone_id", "")

	v.SetDefault("enable_compression", true)

	// Neutral CORS defaults (off by default; empty lists; no credentials; no max-age)
	v.SetDefault("enable_cors", false)
	v.SetDefault("cors_allowed_origins", []string{})
	v.SetDefault("cors_allowed_methods", []string{})
	v.SetDefault("cors_allowed_headers", []string{})
	v.SetDefault("cors_exposed_headers", []string{})
	v.SetDefault("cors_allow_credentials", false)
	v.SetDefault("cors_max_age", 0)

	v.SetDefault("ingest_api_key", "")
	v.SetDefault("admin_api_key", "")
}

// normalizeListKeys coerces JSON-string values into []string for the given keys.
// It also converts []interface{} (from some file formats) into []string.
// If the value is empty string, it leaves it as-is (the default will apply).
func normalizeListKeys(v *viper.Viper, keys ...string) error {
	for _, key := range keys {
		val := v.Get(key)
		switch t := val.(type) {
		case string:
			s := strings.TrimSpace(t)
			if s == "" {
				continue
			}
			// Expect JSON array
			var arr []string
			if err := json.Unmarshal([]byte(s), &arr); err != nil {
				return fmt.Errorf("config key %q expects a JSON array string, got %q: %w", key, s, err)
			}
			v.Set(key, arr)
		case []interface{}:
			arr := make([]string, 0, len(t))
			for _, e := range t {
				arr = append(arr, fmt.Sprint(e))
			}
			v.Set(key, arr)
		case []string, nil:
			// already correct or unset
		default:
			// Unexpected type (e.g., number/bool). Let mapstructure handle, but warn.
			zap.L().Warn("unexpected type for list key; expected JSON array/string",
				zap.String("key", key), zap.Any("value", t))
		}
	}
	return nil
}

func validateConfig(cfg Config) error {
	var missing []string
	var invalid []string

	// Required: Ingest API key
	if s := strings.TrimSpace(cfg.IngestAPIKey); s == "" {
		missing = append(missing, "STRATALOG_INGEST_API_KEY (or --ingest_api_key)")
	}

	// Mongo URI: light, stdlib-only validation
	if err := validateMongoURI(cfg.MongoURI); err != nil {
		invalid = append(invalid, "mongo_uri: "+err.Error())
	}

	// Let’s Encrypt and TLS consistency
	if cfg.UseLetsEncrypt && !cfg.UseHTTPS {
		invalid = append(invalid, "use_lets_encrypt=true requires use_https=true")
	}
	if cfg.UseLetsEncrypt && (strings.TrimSpace(cfg.CertFile) != "" || strings.TrimSpace(cfg.KeyFile) != "") {
		invalid = append(invalid, "use_lets_encrypt=true cannot be combined with cert_file/key_file")
	}

	if cfg.UseLetsEncrypt {
		chal := strings.ToLower(strings.TrimSpace(cfg.LetsEncryptChallenge))
		if chal == "" {
			chal = "http-01"
		}
		switch chal {
		case "http-01":
			// HTTP-01: same requirements as before.
			if strings.TrimSpace(cfg.Domain) == "" {
				missing = append(missing, "STRATALOG_DOMAIN (or --domain) for Let's Encrypt http-01")
			}
			if s := strings.TrimSpace(cfg.LetsEncryptEmail); s == "" {
				missing = append(missing, "STRATALOG_LETS_ENCRYPT_EMAIL (or --lets_encrypt_email)")
			} else if !strings.Contains(cfg.LetsEncryptEmail, "@") {
				invalid = append(invalid, "lets_encrypt_email must look like an email address")
			}
		case "dns-01":
			// DNS-01: still need domain + email + hosted zone ID.
			if strings.TrimSpace(cfg.Domain) == "" {
				missing = append(missing, "STRATALOG_DOMAIN (or --domain) for Let's Encrypt dns-01")
			}
			if s := strings.TrimSpace(cfg.LetsEncryptEmail); s == "" {
				missing = append(missing, "STRATALOG_LETS_ENCRYPT_EMAIL (or --lets_encrypt_email)")
			} else if !strings.Contains(cfg.LetsEncryptEmail, "@") {
				invalid = append(invalid, "lets_encrypt_email must look like an email address")
			}
			if strings.TrimSpace(cfg.Route53HostedZoneID) == "" {
				missing = append(missing, "STRATALOG_ROUTE53_HOSTED_ZONE_ID (or --route53_hosted_zone_id) for dns-01")
			}
		default:
			invalid = append(invalid, "lets_encrypt_challenge must be \"http-01\" or \"dns-01\"")
		}
	}

	// Manual TLS requirements
	if cfg.UseHTTPS && !cfg.UseLetsEncrypt {
		if strings.TrimSpace(cfg.CertFile) == "" || strings.TrimSpace(cfg.KeyFile) == "" {
			missing = append(missing, "STRATALOG_CERT_FILE and STRATALOG_KEY_FILE (or --cert_file/--key_file) for manual TLS")
		}
	}

	// Port sanity
	if cfg.HTTPPort <= 0 || cfg.HTTPPort > 65535 {
		invalid = append(invalid, "HTTP_PORT must be in 1..65535 (got "+strconv.Itoa(cfg.HTTPPort)+")")
	}
	if cfg.HTTPSPort <= 0 || cfg.HTTPSPort > 65535 {
		invalid = append(invalid, "HTTPS_PORT must be in 1..65535 (got "+strconv.Itoa(cfg.HTTPSPort)+")")
	}

	// Port collisions / HTTPS specifics
	if cfg.UseHTTPS {
		if cfg.HTTPPort == cfg.HTTPSPort {
			invalid = append(invalid, "http_port and https_port cannot be equal when use_https=true")
		}
		// Even with dns-01 it's still a bad idea for https_port to be 80.
		if cfg.HTTPSPort == 80 {
			invalid = append(invalid, "https_port cannot be 80")
		}
	}

	// CORS sanity
	if cfg.EnableCORS {
		if len(cfg.CORSAllowedOrigins) == 0 {
			missing = append(missing, "CORS: cors_allowed_origins (JSON array) required when enable_cors=true")
		}
		if len(cfg.CORSAllowedMethods) == 0 {
			missing = append(missing, "CORS: cors_allowed_methods (JSON array) required when enable_cors=true")
		}
		for _, o := range cfg.CORSAllowedOrigins {
			if o == "*" && cfg.CORSAllowCredentials {
				invalid = append(invalid, `CORS: cannot use "*" in cors_allowed_origins when cors_allow_credentials=true`)
				break
			}
		}
		if cfg.CORSMaxAge < 0 {
			invalid = append(invalid, "CORS: cors_max_age must be >= 0")
		}
	}

	if len(missing) == 0 && len(invalid) == 0 {
		return nil
	}

	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "missing: "+strings.Join(missing, ", "))
	}
	if len(invalid) > 0 {
		parts = append(parts, "invalid: "+strings.Join(invalid, ", "))
	}
	return fmt.Errorf("configuration errors: %s", strings.Join(parts, " | "))
}

// validateMongoURI does a lightweight shape check without pulling in the driver.
// Accepts "mongodb://" and "mongodb+srv://" and requires a non-empty host.
// Also rejects CR/LF to avoid header-splitting shenanigans if this ever leaks.
func validateMongoURI(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("empty")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return fmt.Errorf("contains CR/LF")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	switch u.Scheme {
	case "mongodb", "mongodb+srv":
	default:
		return fmt.Errorf(`scheme must be "mongodb" or "mongodb+srv" (got %q)`, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

func (c Config) redactedCopy() Config {
	cp := c

	// Redact API keys
	if cp.IngestAPIKey != "" {
		cp.IngestAPIKey = "****"
	}
	if cp.AdminAPIKey != "" {
		cp.AdminAPIKey = "****"
	}

	// Redact password in Mongo URI, if any
	cp.MongoURI = redactMongoURI(cp.MongoURI)

	return cp
}

func redactMongoURI(s string) string {
	if s == "" {
		return s
	}
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	username := u.User.Username()
	if _, hasPwd := u.User.Password(); hasPwd {
		u.User = url.UserPassword(username, "****")
	} else {
		u.User = url.User(username)
	}
	return u.String()
}

// parseDurationFlexible accepts strings like "90s"/"2m", numeric seconds, or time.Duration.
// Returns def on empty/unknown types; returns def + error on invalid strings.
func parseDurationFlexible(raw interface{}, def time.Duration) (time.Duration, error) {
	switch t := raw.(type) {
	case time.Duration:
		if t <= 0 {
			return def, fmt.Errorf("duration must be >0")
		}
		return t, nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return def, nil
		}
		if d, err := time.ParseDuration(s); err == nil {
			if d <= 0 {
				return def, fmt.Errorf("duration must be >0")
			}
			return d, nil
		}
		// Allow plain seconds in string form, e.g. "120"
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			if n <= 0 {
				return def, fmt.Errorf("seconds must be >0")
			}
			return time.Duration(n) * time.Second, nil
		}
		return def, fmt.Errorf("cannot parse duration %q", s)
	case int:
		if t <= 0 {
			return def, fmt.Errorf("seconds must be >0")
		}
		return time.Duration(t) * time.Second, nil
	case int32:
		if t <= 0 {
			return def, fmt.Errorf("seconds must be >0")
		}
		return time.Duration(int64(t)) * time.Second, nil
	case int64:
		if t <= 0 {
			return def, fmt.Errorf("seconds must be >0")
		}
		return time.Duration(t) * time.Second, nil
	case float64:
		if t <= 0 {
			return def, fmt.Errorf("seconds must be >0")
		}
		return time.Duration(t * float64(time.Second)), nil
	default:
		// Unknown type (nil, bool, etc.) – use default, no error
		return def, nil
	}
}

// internal/app/bootstrap/appconfig.go
package bootstrap

import "time"

// AppConfig holds service-specific configuration for this WAFFLE app.
//
// These values come from environment variables, configuration files, or
// command-line flags (loaded in LoadConfig). They represent *app-level*
// configuration, not WAFFLE core configuration.
//
// WAFFLE's CoreConfig handles framework-level settings like:
//   - HTTP/HTTPS ports and TLS configuration
//   - Logging level and format
//   - CORS settings
//   - Request body size limits
//   - Database connection timeouts
//
// AppConfig is where you put everything specific to YOUR application:
//   - Database connection strings (MongoDB URI, etc.)
//   - External service API keys and endpoints
//   - Feature flags and application modes
//   - Business logic configuration
//   - Default values for your domain
//
// Add fields here as your application grows. The struct is passed to
// most lifecycle hooks, so any configuration needed during startup,
// request handling, or shutdown should live here.
type AppConfig struct {
	// MongoDB connection configuration
	MongoURI         string // MongoDB connection string (e.g., mongodb://localhost:27017)
	MongoDatabase    string // Database name within MongoDB
	MongoMaxPoolSize uint64 // Maximum connections in pool (default: 100)
	MongoMinPoolSize uint64 // Minimum connections to keep warm (default: 10)

	// Session management configuration
	SessionKey    string        // Secret key for signing session cookies (must be strong in production)
	SessionName   string        // Cookie name for sessions (default: strata-session)
	SessionDomain string        // Cookie domain (blank means current host)
	SessionMaxAge time.Duration // Maximum session cookie lifetime (default: 24h)

	// Idle logout configuration
	IdleLogoutEnabled bool          // Enable automatic logout after idle time
	IdleLogoutTimeout time.Duration // Duration of inactivity before logout (default: 30m)
	IdleLogoutWarning time.Duration // Time before logout to show warning (default: 5m)

	// Rate limiting configuration
	RateLimitEnabled       bool          // Enable rate limiting for login attempts (default: true)
	RateLimitLoginAttempts int           // Max failed login attempts before lockout (default: 5)
	RateLimitLoginWindow   time.Duration // Time window for counting failed attempts (default: 15m)
	RateLimitLoginLockout  time.Duration // Lockout duration after exceeding limit (default: 15m)

	// CSRF protection configuration
	CSRFKey string // Secret key for CSRF token signing (32 bytes, must be strong in production)

	// API key authentication (for external API consumers)
	// When set, enables Bearer token authentication for /api/* routes.
	// Leave empty to disable API key authentication.
	APIKey string

	// File storage configuration
	StorageType      string // Storage backend: "local" or "s3"
	StorageLocalPath string // Local storage path (e.g., "./uploads")
	StorageLocalURL  string // URL prefix for serving local files (e.g., "/files")

	// S3/CloudFront configuration (only used if StorageType is "s3")
	StorageS3Region    string // AWS region
	StorageS3Bucket    string // S3 bucket name
	StorageS3Prefix    string // Key prefix (e.g., "uploads/")
	StorageCFURL       string // CloudFront distribution URL
	StorageCFKeyPairID string // CloudFront key pair ID
	StorageCFKeyPath   string // Path to CloudFront private key file

	// Email/SMTP configuration
	MailSMTPHost string // SMTP server host (e.g., localhost for Mailpit, email-smtp.us-east-1.amazonaws.com for SES)
	MailSMTPPort int    // SMTP server port (e.g., 1025 for Mailpit, 587 for SES)
	MailSMTPUser string // SMTP username (empty for Mailpit, SES SMTP credentials for AWS)
	MailSMTPPass string // SMTP password
	MailFrom     string // From email address (e.g., noreply@example.com)
	MailFromName string // From display name (e.g., Strata)

	// Base URL for email links (magic links, password reset, etc.)
	BaseURL string // e.g., "https://example.com" or "http://localhost:3000"

	// Email verification settings
	EmailVerifyExpiry time.Duration // How long email verification codes/links are valid (default: 10m)

	// Audit logging configuration
	// Values: "all" (MongoDB + zap), "db" (MongoDB only), "log" (zap only), "off" (disabled)
	AuditLogAuth  string // Authentication events (login, logout, password, verification)
	AuditLogAdmin string // Admin actions (user CRUD, settings changes)

	// Google OAuth configuration
	GoogleClientID     string // Google OAuth2 client ID
	GoogleClientSecret string // Google OAuth2 client secret

	// Admin seeding configuration
	SeedAdminEmail string // Email of the admin user to create on startup (if set)
	SeedAdminName  string // Name of the admin user to create on startup
}

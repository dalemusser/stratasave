# StrataSave Configuration Guide

StrataSave uses a layered configuration system powered by the Waffle framework. Configuration can be provided through config files, environment variables, or command-line flags.

## Configuration Precedence

Settings are merged with the following precedence (highest wins):

1. Command-line flags
2. Environment variables
3. Config files (`config.toml`, `config.yaml`, or `config.json`)
4. Default values

## Config File Location

Place your config file in the application's working directory:

- `config.toml` (recommended)
- `config.yaml`
- `config.json`

## Configuration Sections

StrataSave configuration is divided into two sections:

1. **Waffle Core Configuration** - Framework-level settings (HTTP, TLS, logging, etc.)
2. **StrataSave Application Configuration** - App-specific settings (MongoDB, sessions, etc.)

---

## Waffle Core Configuration

### Runtime Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `env` | string | `"dev"` | Runtime environment: `"dev"` or `"prod"` |
| `log_level` | string | `"info"` | Logging level: `debug`, `info`, `warn`, `error` |

### HTTP Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `http_port` | int | `8080` | HTTP server port |
| `https_port` | int | `8443` | HTTPS server port |
| `use_https` | bool | `false` | Enable HTTPS |
| `read_timeout` | duration | `"30s"` | Max time to read request |
| `read_header_timeout` | duration | `"10s"` | Max time to read request headers |
| `write_timeout` | duration | `"30s"` | Max time to write response |
| `idle_timeout` | duration | `"120s"` | Max time for keep-alive connections |
| `shutdown_timeout` | duration | `"30s"` | Graceful shutdown timeout |

### TLS Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cert_file` | string | `""` | Path to TLS certificate file |
| `key_file` | string | `""` | Path to TLS private key file |
| `use_lets_encrypt` | bool | `false` | Enable automatic Let's Encrypt certificates |
| `lets_encrypt_email` | string | `""` | Email for Let's Encrypt registration |
| `lets_encrypt_cache_dir` | string | `""` | Directory to cache Let's Encrypt certificates |
| `lets_encrypt_challenge` | string | `"http-01"` | ACME challenge type: `"http-01"` or `"dns-01"` |
| `domain` | string | `""` | Single domain for TLS certificate (backward compatible) |
| `domains` | []string | `[]` | Multiple domains for TLS certificate (e.g., `["example.com", "*.example.com"]`) |
| `route53_hosted_zone_id` | string | `""` | AWS Route53 zone ID (for dns-01 challenge) |
| `acme_directory_url` | string | `""` | Custom ACME directory URL |

> **Note:** Use either `domain` (single domain) or `domains` (multiple domains), not both. Wildcard certificates (e.g., `*.example.com`) require `lets_encrypt_challenge = "dns-01"`.

### CORS Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enable_cors` | bool | `false` | Enable CORS headers |
| `cors_allowed_origins` | []string | `[]` | Allowed origins |
| `cors_allowed_methods` | []string | `[]` | Allowed HTTP methods |
| `cors_allowed_headers` | []string | `[]` | Allowed request headers |
| `cors_exposed_headers` | []string | `[]` | Headers exposed to browser |
| `cors_allow_credentials` | bool | `false` | Allow credentials |
| `cors_max_age` | int | `0` | Preflight cache duration (seconds) |

### Database & Misc Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `db_connect_timeout` | duration | `"10s"` | Database connection timeout |
| `index_boot_timeout` | duration | `"30s"` | Index creation timeout at startup |
| `max_request_body_bytes` | int64 | `10485760` | Max request body size (bytes) |
| `enable_compression` | bool | `true` | Enable response compression |
| `compression_level` | int | `5` | Compression level (1-9) |

---

## StrataSave Application Configuration

### Database Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `mongo_uri` | string | `"mongodb://localhost:27017"` | MongoDB connection URI |
| `mongo_database` | string | `"stratasave"` | MongoDB database name |
| `mongo_max_pool_size` | int | `100` | MongoDB max connection pool size |
| `mongo_min_pool_size` | int | `10` | MongoDB min connection pool size |

### Session Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `session_key` | string | *(dev default)* | Secret key for signing session cookies |
| `session_name` | string | `"stratasave-session"` | Session cookie name |
| `session_domain` | string | `""` | Session cookie domain (blank = current host) |
| `session_max_age` | duration | `"24h"` | Session cookie lifetime (e.g., `24h`, `720h`, `30m`) |

> **Security Note:** The `session_key` must be a strong, random string in production. Never use the default development key in production environments.

### Idle Logout Configuration

StrataSave can automatically log out users who are idle (browser tab open but no interaction). This is useful for security-sensitive deployments where unattended sessions should be terminated.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `idle_logout_enabled` | bool | `false` | Enable automatic logout after idle time |
| `idle_logout_timeout` | duration | `"30m"` | Duration of inactivity before logout |
| `idle_logout_warning` | duration | `"5m"` | Time before logout to show warning banner |

**How it works:**
- "Idle" means the browser tab is open (heartbeat running) but the user hasn't interacted (no clicks, keystrokes, or scrolling)
- When idle time approaches the timeout, a yellow warning banner appears: "You will be logged out due to inactivity in X minute(s). [Stay Logged In]"
- Clicking "Stay Logged In" or any user interaction resets the idle timer
- If the user doesn't interact, they are automatically logged out when the timeout expires

**Example configuration:**
```toml
# Log out users after 15 minutes of inactivity, warn 2 minutes before
idle_logout_enabled = true
idle_logout_timeout = "15m"
idle_logout_warning = "2m"
```

> **Note:** Idle logout is disabled by default. When disabled, sessions remain active as long as the browser tab is open (within `session_max_age`).

### Rate Limiting Configuration

StrataSave includes configurable rate limiting to protect against brute force login attacks. Rate limiting is per-login_id (not per-IP), which allows many users from the same IP address (like students in a school) to log in without blocking each other.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `rate_limit_enabled` | bool | `true` | Enable rate limiting for login attempts |
| `rate_limit_login_attempts` | int | `5` | Max failed login attempts before lockout |
| `rate_limit_login_window` | duration | `"15m"` | Time window for counting failed attempts |
| `rate_limit_login_lockout` | duration | `"15m"` | Lockout duration after exceeding limit |

**How it works:**
- Each login_id (username or email) has its own rate limit counter
- After `rate_limit_login_attempts` failed attempts within `rate_limit_login_window`, that login_id is locked out
- The lockout lasts for `rate_limit_login_lockout` duration
- A successful login clears the rate limit counter for that login_id
- Rate limit records are automatically cleaned up after 24 hours

**Example configuration:**
```toml
# 3 attempts in 5 minutes, then 10 minute lockout
rate_limit_enabled = true
rate_limit_login_attempts = 3
rate_limit_login_window = "5m"
rate_limit_login_lockout = "10m"
```

**To disable rate limiting:**
```toml
rate_limit_enabled = false
```

> **Note:** Rate limiting is enabled by default with 5 attempts per 15 minutes.

### Security Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `csrf_key` | string | *(dev default)* | CSRF token signing key (32+ chars in production) |
| `api_key` | string | `""` | API key for external API access (empty = disabled) |

---

## Email/SMTP Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `mail_smtp_host` | string | `"localhost"` | SMTP server hostname |
| `mail_smtp_port` | int | `1025` | SMTP server port |
| `mail_smtp_user` | string | `""` | SMTP username |
| `mail_smtp_pass` | string | `""` | SMTP password |
| `mail_from` | string | `"noreply@example.com"` | From email address |
| `mail_from_name` | string | `"Strata"` | From display name |
| `base_url` | string | `"http://localhost:8080"` | Base URL for magic links |
| `email_verify_expiry` | duration | `"10m"` | Email verification code/link expiry |

### Email Configuration for Development

For local development, use [Mailpit](https://github.com/axllent/mailpit) to capture emails:

```bash
# Install Mailpit (macOS)
brew install mailpit

# Run Mailpit
mailpit
# or
brew services start mailpit
```

- **SMTP**: localhost:1025 (default, no config needed)
- **Web UI**: http://localhost:8025

### Email Configuration for Production

For production, configure a real SMTP server:

```toml
mail_smtp_host = "email-smtp.us-east-1.amazonaws.com"
mail_smtp_port = 587
mail_smtp_user = "your-ses-smtp-user"
mail_smtp_pass = "your-ses-smtp-password"
mail_from = "noreply@yourdomain.com"
mail_from_name = "Your App Name"
base_url = "https://yourdomain.com"
email_verify_expiry = "10m"
```

---

## File Storage Configuration

StrataSave supports two storage backends for uploaded files:

1. **Local storage** - Files stored on the local filesystem and served by the application
2. **S3/CloudFront** - Files stored in AWS S3 with signed CloudFront URLs for secure delivery

### Storage Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `storage_type` | string | `"local"` | Storage backend: `"local"` or `"s3"` |
| `storage_local_path` | string | `"./uploads"` | Local filesystem path for uploaded files |
| `storage_local_url` | string | `"/files"` | URL prefix for serving local files |

### S3/CloudFront Settings

Required when `storage_type = "s3"`:

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `storage_s3_region` | string | `""` | AWS region (e.g., `"us-east-1"`) |
| `storage_s3_bucket` | string | `""` | S3 bucket name |
| `storage_s3_prefix` | string | `"uploads/"` | Key prefix for uploaded files |
| `storage_cf_url` | string | `""` | CloudFront distribution URL |
| `storage_cf_keypair_id` | string | `""` | CloudFront key pair ID for signed URLs |
| `storage_cf_key_path` | string | `""` | Path to CloudFront private key file (.pem) |

---

## Audit Logging Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `audit_log_auth` | string | `"all"` | Auth event logging: `"all"`, `"db"`, `"log"`, or `"off"` |
| `audit_log_admin` | string | `"all"` | Admin event logging: `"all"`, `"db"`, `"log"`, or `"off"` |

Values:
- `"all"` - Log to both MongoDB and zap logger
- `"db"` - Log to MongoDB only
- `"log"` - Log to zap logger only
- `"off"` - Disable logging

---

## Google OAuth Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `google_client_id` | string | `""` | Google OAuth2 client ID |
| `google_client_secret` | string | `""` | Google OAuth2 client secret |

To enable Google OAuth:
1. Create a project in the [Google Cloud Console](https://console.cloud.google.com/)
2. Enable the Google+ API
3. Create OAuth 2.0 credentials
4. Set the authorized redirect URI to `{base_url}/auth/google/callback`

---

## Admin Seeding Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `seed_admin_email` | string | `""` | Email of admin user to create on startup |
| `seed_admin_name` | string | `"Admin"` | Name of admin user to create on startup |

When `seed_admin_email` is set, StrataSave will create an admin user with that email on startup if one doesn't already exist.

---

## Runtime Admin Settings (Database)

Some settings are stored in the database and configured via the admin UI at `/settings`. These settings can be changed at runtime without restarting the server.

### Email Notification Settings

Configure which email notifications are sent to users. All notifications are **disabled by default** (opt-in).

| Setting | Default | Description |
|---------|---------|-------------|
| Send welcome email on user create | `false` | Send welcome email when admin creates a new user |
| Send notification on disable | `false` | Send notification when user account is disabled |
| Send notification on enable | `false` | Send notification when user account is enabled |
| Send welcome email on invitation accept | `false` | Send welcome email after invitation is accepted |

**To configure:** Navigate to `/settings` as an admin and scroll to the "Email Notifications" section.

**Note:** Email notifications require SMTP to be configured (see [Email/SMTP Configuration](#emailsmtp-configuration)).

---

## Environment Variables

Configuration can be overridden using environment variables. Strata uses the `STRATASAVE_` prefix for all environment variables.

Examples:
```bash
export STRATASAVE_HTTP_PORT=3000
export STRATASAVE_LOG_LEVEL=debug
export STRATASAVE_ENV=prod
export STRATASAVE_MONGO_URI="mongodb://user:pass@dbserver:27017"
export STRATASAVE_SESSION_KEY="your-production-secret-key"
export STRATASAVE_SESSION_MAX_AGE=24h

# Optional: Enable idle logout
export STRATASAVE_IDLE_LOGOUT_ENABLED=true
export STRATASAVE_IDLE_LOGOUT_TIMEOUT=30m
export STRATASAVE_IDLE_LOGOUT_WARNING=5m

# Rate limiting (enabled by default, configure to adjust)
# export STRATASAVE_RATE_LIMIT_ENABLED=true
# export STRATASAVE_RATE_LIMIT_LOGIN_ATTEMPTS=5
# export STRATASAVE_RATE_LIMIT_LOGIN_WINDOW=15m
# export STRATASAVE_RATE_LIMIT_LOGIN_LOCKOUT=15m
```

---

## Example: Local Development Configuration

### Prerequisites

- MongoDB running on `localhost:27017`
- Database `stratasave` (will be created automatically)

### config.toml

```toml
# StrataSave Local Development Configuration
# Place this file in the application root directory

# =============================================================================
# Waffle Core Configuration
# =============================================================================

# Runtime
env = "dev"
log_level = "debug"

# HTTP Server
http_port = 8080
https_port = 8443
use_https = false

# Server Timeouts
read_timeout = "30s"
read_header_timeout = "10s"
write_timeout = "30s"
idle_timeout = "120s"
shutdown_timeout = "30s"

# TLS (disabled for local dev)
# cert_file = ""
# key_file = ""
# use_lets_encrypt = false

# CORS (not needed for same-origin local dev)
enable_cors = false

# Database Timeouts
db_connect_timeout = "10s"
index_boot_timeout = "30s"

# HTTP Behavior
max_request_body_bytes = 10485760  # 10 MB

# Compression
enable_compression = true
compression_level = 5

# =============================================================================
# StrataSave Application Configuration
# =============================================================================

# MongoDB - local instance
mongo_uri = "mongodb://localhost:27017"
mongo_database = "stratasave"

# Session Configuration
# WARNING: Change session_key in production!
session_key = "dev-only-change-me-please-0123456789ABCDEF"
session_name = "stratasave-session"
session_domain = ""
session_max_age = "24h"

# Idle Logout (disabled by default)
# idle_logout_enabled = false
# idle_logout_timeout = "30m"
# idle_logout_warning = "5m"

# CSRF Protection
# WARNING: Change csrf_key in production!
csrf_key = "dev-only-csrf-key-please-change-0123456789"

# File Storage (local for development)
storage_type = "local"
storage_local_path = "./uploads"
storage_local_url = "/files"

# Email (Mailpit for development)
mail_smtp_host = "localhost"
mail_smtp_port = 1025
mail_from = "noreply@example.com"
mail_from_name = "StrataSave"
base_url = "http://localhost:8080"
email_verify_expiry = "10m"

# Audit Logging
audit_log_auth = "all"
audit_log_admin = "all"
```

### Running the Application

```bash
# Start MongoDB (if not already running)
brew services start mongodb-community  # macOS with Homebrew
# or
mongod --dbpath /path/to/data          # Manual start

# Run StrataSave
go run ./cmd/stratasave
```

The application will be available at `http://localhost:8080`.

---

## Production Considerations

When deploying to production:

1. **Set `env = "prod"`** - Enables production optimizations
2. **Use strong keys** - Generate `session_key` and `csrf_key` with: `openssl rand -hex 32`
3. **Enable HTTPS** - Either with your own certificates or Let's Encrypt
4. **Use environment variables for secrets** - Don't commit secrets to config files
5. **Set appropriate timeouts** - Adjust based on your infrastructure
6. **Configure CORS if needed** - For API access from different origins

### Example Production Environment Variables

```bash
# All settings use the STRATASAVE_ prefix
export STRATASAVE_ENV=prod
export STRATASAVE_LOG_LEVEL=info
export STRATASAVE_USE_HTTPS=true
export STRATASAVE_USE_LETS_ENCRYPT=true
export STRATASAVE_LETS_ENCRYPT_EMAIL=admin@yourdomain.com
export STRATASAVE_DOMAIN=yourdomain.com

export STRATASAVE_MONGO_URI="mongodb://user:password@mongo.yourdomain.com:27017/stratasave?authSource=admin"
export STRATASAVE_SESSION_KEY="$(openssl rand -hex 32)"
export STRATASAVE_SESSION_DOMAIN=".yourdomain.com"
export STRATASAVE_SESSION_MAX_AGE=24h
export STRATASAVE_CSRF_KEY="$(openssl rand -hex 32)"

# Optional: Enable idle logout for security-sensitive deployments
# export STRATASAVE_IDLE_LOGOUT_ENABLED=true
# export STRATASAVE_IDLE_LOGOUT_TIMEOUT=30m
# export STRATASAVE_IDLE_LOGOUT_WARNING=5m

# Rate limiting (enabled by default, configure to disable or adjust)
# export STRATASAVE_RATE_LIMIT_ENABLED=true
# export STRATASAVE_RATE_LIMIT_LOGIN_ATTEMPTS=5
# export STRATASAVE_RATE_LIMIT_LOGIN_WINDOW=15m
# export STRATASAVE_RATE_LIMIT_LOGIN_LOCKOUT=15m

# Email configuration
export STRATASAVE_MAIL_SMTP_HOST=email-smtp.us-east-1.amazonaws.com
export STRATASAVE_MAIL_SMTP_PORT=587
export STRATASAVE_MAIL_SMTP_USER=your-ses-smtp-user
export STRATASAVE_MAIL_SMTP_PASS=your-ses-smtp-password
export STRATASAVE_MAIL_FROM=noreply@yourdomain.com
export STRATASAVE_MAIL_FROM_NAME="Your App Name"
export STRATASAVE_BASE_URL=https://yourdomain.com
```

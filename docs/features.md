# Strata Application Starter - Features & Capabilities

Strata is a Go web application starter/template that provides authentication, user management, content management, and administrative features out of the box. Built on the Waffle framework with MongoDB, Chi router, HTMX, and Tailwind CSS.

---

## Table of Contents

1. [Authentication](#authentication)
2. [User Management](#user-management)
3. [Content Management](#content-management)
4. [File Management](#file-management)
5. [Site Administration](#site-administration)
6. [Audit & Monitoring](#audit--monitoring)
7. [Data Layer](#data-layer)
8. [System Utilities](#system-utilities)
9. [Frontend Resources](#frontend-resources)
10. [Configuration](#configuration)
11. [Extending Strata](#extending-strata)

---

## Authentication

Strata provides a flexible, multi-method authentication system.

### Supported Auth Methods

| Method | Description |
|--------|-------------|
| **Password** | Traditional email/password authentication with bcrypt hashing |
| **Email** | Passwordless authentication via one-time codes or magic links |
| **Google OAuth** | OAuth2 integration with Google accounts |
| **Trust** | Development-only method for quick login without credentials |

### Security Features

- **Password Requirements**: Minimum 8 characters, mixed case, special characters
- **Rate Limiting**: Configurable limits on failed login attempts (default: 5 attempts in 15 minutes, 15-minute lockout)
- **Session Management**: Secure cookie-based sessions with configurable expiry
- **CSRF Protection**: Built-in CSRF tokens on all state-changing requests
- **OAuth State Validation**: Prevents CSRF in OAuth flows

### Password Recovery

- Email-based password reset with secure tokens
- Configurable token expiry (default: 10 minutes)
- Single-use tokens
- Email confirmation after password change

### Session Features

- Device tracking (IP address, User Agent)
- Active session list in user profile
- Revoke individual sessions or all except current
- Idle logout with configurable timeout and warning

---

## User Management

### User Roles

| Role | Capabilities |
|------|--------------|
| **Admin** | Full system access, user management, settings, audit logs |
| **User** | Default role with access to dashboard and profile |

Additional roles can be added by extending the role constants.

### User Attributes

- Full name with case-insensitive search support
- Login ID (email or username) with case-insensitive matching
- Optional email address
- Authentication method
- Account status (active/disabled)
- Theme preference (light/dark/system)

### Admin User Management

- Create users with any authentication method
- Edit user details and roles
- Enable/disable user accounts
- Reset passwords to temporary values
- Delete users
- Paginated list with search and status filtering

---

## Content Management

### Dynamic Pages

Four built-in editable pages:

| Page | Slug | Purpose |
|------|------|---------|
| About | `/about` | Company/site information |
| Contact | `/contact` | Contact information |
| Terms | `/terms` | Terms of Service |
| Privacy | `/privacy` | Privacy Policy |

Features:
- TipTap rich text editor integration
- HTML sanitization for XSS prevention
- Admin-only editing
- Tracks who last updated and when

### Landing Page

- Customizable title and content
- Rich HTML support with sanitization
- Editable from site settings

### Footer

- Custom HTML footer content
- Site-wide display
- Configurable in settings

---

## File Management

A complete document library system with nested folder support.

### Capabilities

| Feature | Description |
|---------|-------------|
| **Folder Hierarchy** | Unlimited nesting depth with breadcrumb navigation |
| **File Upload** | Up to 32MB per file |
| **File Metadata** | Name, description, size, content type |
| **Search & Filter** | Filter by content type, search by name |
| **Sorting** | Sort by name or date |
| **Inline Viewing** | View images, PDFs, videos, audio in browser |
| **Download** | Direct file download |

### Storage Backends

- **Local Filesystem**: Default, stores in configurable directory
- **Amazon S3**: With optional CloudFront CDN integration

Files are stored with unique paths: `files/YYYY/MM/uuid-extension`

### Access Control

- All authenticated users can browse and download
- Admin-only: create folders, upload files, edit, delete
- Recursive folder deletion cleans up all contents

---

## Site Administration

### Site Settings

| Setting | Description |
|---------|-------------|
| Site Name | Displayed in header and titles |
| Logo | Upload/remove site logo |
| Landing Title | Homepage headline |
| Landing Content | Homepage body content |
| Footer HTML | Custom footer content |

### Announcements

System-wide announcements displayed to all users.

| Feature | Description |
|---------|-------------|
| Types | Info, Warning, Success, Error |
| Scheduling | Optional start and end dates |
| Dismissible | Users can dismiss if enabled |
| Admin Management | Full CRUD interface |

### User Invitations

- Admin-generated invitation links
- Email delivery of invitations
- 7-day expiry (configurable)
- Single-use tokens
- Direct registration from invitation link

---

## Audit & Monitoring

### Audit Logging

Comprehensive security event logging with configurable output (database, log file, both, or off).

#### Authentication Events

- Login success/failure
- Logout
- Password changes
- Email verification
- Password reset requests

#### Admin Action Events

- User create/update/delete
- Settings changes
- File operations
- Page edits

#### Event Data Captured

- Timestamp
- Actor (who performed the action)
- Target user (if applicable)
- IP address
- User Agent
- Success/failure status
- Additional details

### Activity Tracking

- User activity event logging
- Page view tracking
- Session activity monitoring

### System Status

Health check page showing:
- MongoDB connection status
- Database name
- Configuration overview (secrets masked)
- System health metrics

### Health Endpoints

- `/health` - Load balancer health check
- Returns system status for orchestrators

---

## Data Layer

### Database

MongoDB with:
- Connection pooling (configurable min/max)
- Indexed fields for performance
- Case/diacritic-insensitive search fields
- Transaction support

### Store Packages

| Store | Purpose |
|-------|---------|
| `users` | User persistence and queries |
| `sessions` | Active session tracking |
| `emailverify` | Email verification tokens |
| `passwordreset` | Password reset tokens |
| `ratelimit` | Login attempt tracking |
| `oauthstate` | OAuth state validation |
| `pages` | Dynamic page content |
| `settings` | Site configuration |
| `file` | File metadata |
| `folder` | Folder hierarchy |
| `announcement` | Announcements |
| `audit` | Audit event log |
| `activity` | User activity events |
| `invitation` | User invitations |
| `logins` | Login history |

---

## System Utilities

### Authentication & Authorization

| Package | Purpose |
|---------|---------|
| `auth` | Session management, middleware |
| `authutil` | Password hashing and validation |
| `authz` | Role-based access control |

### Security

| Package | Purpose |
|---------|---------|
| `htmlsanitize` | XSS prevention for user HTML |
| `apicors` | CORS middleware for APIs |

### Data Processing

| Package | Purpose |
|---------|---------|
| `inputval` | Input validation rules |
| `normalize` | Data normalization (emails, names) |
| `jsonutil` | JSON response helpers |

### Communication

| Package | Purpose |
|---------|---------|
| `mailer` | SMTP email delivery |
| `network` | IP extraction, proxy awareness |

### Infrastructure

| Package | Purpose |
|---------|---------|
| `viewdata` | Template context building |
| `indexes` | Database index management |
| `tasks` | Background job scheduling |
| `timezones` | Timezone handling |
| `timeouts` | Request timeout management |
| `txn` | MongoDB transaction helpers |
| `seeding` | Database seed data |

---

## Frontend Resources

### Templates

Templates organized by feature in `resources/templates/`:

```
templates/
├── layouts/          # Base layouts
├── home/            # Landing page
├── login/           # Auth forms
├── dashboard/       # Dashboards
├── profile/         # User profile
├── systemusers/     # User management
├── pages/           # Page editor
├── settings/        # Site settings
├── files/           # File browser
├── announcements/   # Announcements
├── invitations/     # Invitations
├── auditlog/        # Audit viewer
├── activity/        # Activity dashboard
├── errors/          # Error pages
└── status/          # System status
```

### Assets

- **CSS**: Tailwind CSS with custom configuration
- **JavaScript**: HTMX integration, theme switching, modals

### UI Features

- Dark mode support with user preference
- HTMX for dynamic updates without page reloads
- Modal dialogs for confirmations and forms
- Pagination with configurable page sizes
- Flash messages for feedback
- Breadcrumb navigation

---

## Configuration

Configuration via environment variables with `STRATA_` prefix.

### Database

| Variable | Description |
|----------|-------------|
| `mongo_uri` | MongoDB connection string |
| `mongo_database` | Database name |
| `mongo_max_pool_size` | Maximum connections |
| `mongo_min_pool_size` | Minimum connections |

### Sessions

| Variable | Description |
|----------|-------------|
| `session_key` | Encryption key (32+ chars) |
| `session_name` | Cookie name |
| `session_domain` | Cookie domain |
| `session_max_age` | Session duration |

### Idle Logout

| Variable | Description |
|----------|-------------|
| `idle_logout_enabled` | Enable/disable |
| `idle_logout_timeout` | Timeout duration |
| `idle_logout_warning` | Warning before logout |

### Rate Limiting

| Variable | Description |
|----------|-------------|
| `rate_limit_enabled` | Enable/disable |
| `rate_limit_login_attempts` | Max attempts |
| `rate_limit_login_window` | Time window |
| `rate_limit_login_lockout` | Lockout duration |

### Storage

| Variable | Description |
|----------|-------------|
| `storage_type` | `local` or `s3` |
| `storage_local_path` | Local storage directory |
| `storage_local_url` | URL prefix for local files |
| `storage_s3_region` | AWS region |
| `storage_s3_bucket` | S3 bucket name |
| `storage_s3_prefix` | Key prefix |
| `storage_cf_url` | CloudFront distribution URL |
| `storage_cf_keypair_id` | CloudFront key pair ID |
| `storage_cf_key_path` | CloudFront private key path |

### Email

| Variable | Description |
|----------|-------------|
| `mail_smtp_host` | SMTP server hostname |
| `mail_smtp_port` | SMTP port |
| `mail_smtp_user` | SMTP username |
| `mail_smtp_pass` | SMTP password |
| `mail_from` | From email address |
| `mail_from_name` | From display name |

### OAuth

| Variable | Description |
|----------|-------------|
| `google_client_id` | Google OAuth client ID |
| `google_client_secret` | Google OAuth client secret |

### Audit

| Variable | Description |
|----------|-------------|
| `audit_log_auth` | Auth event output (db/log/both/off) |
| `audit_log_admin` | Admin event output |

### Seeding

| Variable | Description |
|----------|-------------|
| `seed_admin_email` | Initial admin email |
| `seed_admin_name` | Initial admin name |

---

## Extending Strata

### Adding a New Feature

1. Create package: `internal/app/features/<feature>/`
2. Define Handler struct:
   ```go
   type Handler struct {
       db     *mongo.Database
       store  *<feature>store.Store
       // other dependencies
   }

   func NewHandler(db *mongo.Database) *Handler {
       return &Handler{db: db, store: <feature>store.New(db)}
   }
   ```

3. Define routes:
   ```go
   func (h *Handler) Routes() http.Handler {
       r := chi.NewRouter()
       r.Get("/", h.list)
       r.Post("/", h.create)
       // ...
       return r
   }
   ```

4. Mount in `bootstrap/routes.go`:
   ```go
   r.Mount("/<feature>", features.<feature>.NewHandler(db).Routes())
   ```

5. Create templates in `resources/templates/<feature>/`

### Adding a New Store

1. Create package: `internal/app/store/<entity>/`
2. Define store struct and model:
   ```go
   type Store struct {
       c *mongo.Collection
   }

   func New(db *mongo.Database) *Store {
       return &Store{c: db.Collection("<collection>")}
   }
   ```

3. Implement CRUD methods with `context.Context` as first parameter
4. Add indexes in `system/indexes/indexes.go`

### Adding a New Role

1. Add constant in domain models
2. Update validation in `normalize` package
3. Create role-specific dashboard template
4. Add middleware checks where needed:
   ```go
   r.With(sessionMgr.RequireRole("newrole")).Get("/path", handler)
   ```

### Adding a New Auth Method

1. Add to `AllAuthMethods` in domain models
2. Create feature handler for auth flow
3. Add login template
4. Update login logic to handle new method
5. Mount routes in bootstrap

---

## Waffle Framework

Strata is built on the custom Waffle framework which provides:

| Package | Capability |
|---------|------------|
| `waffle/config` | Environment, file, and flag configuration |
| `waffle/middleware` | CORS, timeouts, logging middleware |
| `waffle/pantry/templates` | HTML templating with hot reload |
| `waffle/pantry/storage` | Pluggable file storage (local/S3) |
| `waffle/pantry/mongo` | MongoDB utilities |
| `waffle/pantry/text` | Text utilities (case folding) |
| `waffle/pantry/fileserver` | Static file serving |
| `waffle/pantry/query` | Query parameter parsing |
| `waffle/pantry/urlutil` | URL utilities |
| `waffle/pantry/testing` | Test utilities |

---

## Quick Start

1. Copy Strata as your starting point
2. Configure environment variables (see Configuration)
3. Run `make build` to build
4. Run `make run` to start
5. Access at `http://localhost:8080`
6. Login with seeded admin account or create via invitation

### Development Commands

```bash
make build    # Build the application
make run      # Build and run
make test     # Run tests
make dev      # Run with hot reload
```

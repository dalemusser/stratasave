# StrataSave — Context for Claude Code

## What This Repo Does

StrataSave is a game save data persistence service that stores and manages player save data with backward compatibility, player settings, and progress history. It provides a web-based developer UI for observing and managing save data, along with public APIs for game clients to persist save data.

The application is built as a reference implementation / template for building Go-based web services with built-in authentication, user management, settings, and monitoring capabilities. It can be forked and customized for different data persistence services.

## Technology Stack

- **Language:** Go 1.24.1
- **Framework:** waffle (custom Go web framework at `github.com/dalemusser/waffle`)
- **Router:** chi/v5 (HTTP router)
- **Database:** MongoDB 7 (via docker-compose)
- **Frontend:** HTMX + Tailwind CSS (standalone CLI)
- **Authentication:** OAuth2 (Google), custom session management (gorilla/sessions)
- **Deployment:** Docker, Linux 386 target, HTTPS with Let's Encrypt support
- **Email:** MailHog for development (SMTP)
- **Logging:** Uber Zap (structured logging)

## Folder Structure

```
stratasave/
├── cmd/stratasave/          # Entry point (main.go)
├── internal/
│   ├── app/
│   │   ├── bootstrap/       # App initialization, config, routes, hooks
│   │   ├── features/        # Feature modules (activity, apikeys, dashboard, etc.)
│   │   ├── store/           # MongoDB data stores (users, sessions, audit, etc.)
│   │   ├── system/          # System utilities (timezones, constants)
│   │   └── resources/
│   │       ├── templates/   # HTML templates (HTMX-powered)
│   │       └── assets/      # CSS, JS
│   ├── domain/
│   │   └── models/          # Domain model definitions
│   └── testutil/            # Testing utilities
├── docs/                    # Documentation (API, deployment, design patterns)
├── scripts/                 # Build and utility scripts
├── config.toml              # Runtime configuration
├── config.example.toml      # Configuration template
├── docker-compose.yml       # Local development services (MongoDB, MailHog)
├── Makefile                 # Build targets (build, run, dev, css, test)
├── Dockerfile               # Docker image for production
├── go.mod / go.sum         # Go dependencies
├── tailwind.config.js       # Tailwind CSS configuration
└── tailwindcss              # Tailwind CLI binary (platform-specific)
```

The organizing principle is **feature-based architecture** where each user-facing feature (activity, dashboard, settings, apikeys, etc.) is self-contained with its own handler, routes, and templates. MongoDB stores are organized by entity (users, sessions, audit, etc.). The waffle framework owns the HTTP lifecycle, database connection, and graceful shutdown.

## Code Patterns & Conventions

- **Naming:** Go conventions (PascalCase for exported, camelCase for unexported). Features use snake_case directory names.
- **Organization:** 
  - Each feature in `internal/app/features/<name>/` with `<name>.go` containing Handler struct and Routes() function
  - Each store in `internal/app/store/<entity>/` with New(db), CRUD methods, Input/Update structs
  - Templates in `internal/app/resources/templates/<feature>/`
- **View Data:** All templates receive a BaseVM with common fields (SiteName, Title, IsLoggedIn, CurrentPath, etc.). Use `viewdata.New(r)` or `viewdata.NewBaseVM(r, db, title, backURL)`.
- **Error Handling:** Custom error feature in `internal/app/features/errors/` with error templates. HTTP errors rendered as pages or JSON responses.
- **Auth:** 
  - `auth.CurrentUser(r)` returns `(*SessionUser, bool)`
  - `sessionMgr.RequireAuth` middleware wraps authenticated routes
  - `sessionMgr.RequireRole("admin")` for role-based access control
  - OAuth2 integration for Google login via `authgoogle` feature
- **Database:** MongoDB with connection pooling, configurable pool sizes. Stores use `context.Context` first parameter, implement CRUD with typed Input/Update structs.

## Key Dependencies & Gotchas

**Custom Waffle Framework:** StrataSave depends on the custom `waffle` Go framework built by Dale. This is not a standard framework like Gin or Echo. The waffle framework handles HTTP lifecycle, config loading (via flags/env/file), graceful shutdown, and provides `app.Run(ctx, hooks)` to execute the full service lifecycle. See `/internal/app/bootstrap/hooks.go` for how startup/shutdown are wired.

**MongoDB Required:** Local development requires MongoDB running (docker-compose handles this). The app will fail to start if MongoDB is unreachable. Connection pooling is configurable via `mongo_max_pool_size` and `mongo_min_pool_size`.

**Tailwind CSS Compilation:** Tailwind is compiled via a standalone CLI binary (`./tailwindcss`), not npm. Must run `make css` or `make css-watch` after CSS changes. The binary is platform-specific; run `make setup-tailwind` if missing.

**Session Key Security:** Default session key in config.example.toml is for development only (`dev-only-change-me-please-0123456789ABCDEF`). Must change to a strong random key in production.

**Large Dependency Graph:** The go.mod has many transitive dependencies from OAuth2, cloud SDKs, telemetry, and MongoDriver. This is expected but makes builds slower and increases binary size.

## How to Run Locally

```bash
# Initial setup
make setup                  # Downloads Tailwind CSS binary

# Copy and customize config
cp config.example.toml config.toml
# Edit config.toml if needed (defaults work for local dev)

# Start MongoDB and MailHog (first time)
docker-compose up -d

# Build Tailwind CSS
make css                    # Or 'make css-watch' for live reload

# Run the app
make dev                    # Uses 'air' for live reload if installed, else 'go run'
# OR
make run                    # Build and run once

# Test
make test
make test-cover            # With coverage report

# Seed admin user
make seed-admin EMAIL=admin@example.com
```

The app starts on http://localhost:8080 by default. MailHog web UI is at http://localhost:8025.

## Related Repos

- **waffle** (`github.com/dalemusser/waffle`) — Custom Go web framework providing HTTP lifecycle, config, database connections, graceful shutdown. StrataSave is built on top of waffle.
- **stratahub** — Another waffle-based application using similar patterns (features, stores, templates).

## Notes for Claude

1. **Follow waffle conventions:** When adding features, look at existing features like `home`, `dashboard`, or `profile` as templates. Every feature needs a Handler struct, NewHandler func, and Routes() returning http.Handler.

2. **Template context:** All templates receive BaseVM. Access via `.SiteName`, `.IsLoggedIn`, `.CurrentPath`, etc. View models are defined in the feature's main file.

3. **Auth is built-in:** Most features don't need to worry about auth; use middleware. OAuth2 setup is in `authgoogle` feature.

4. **MongoDB is strict:** Stores use typed structs (Input, Update) for complex operations. No raw string queries.

5. **Config via environment:** Prefer environment variables over editing config.toml. Use `STRATASAVE_` prefix. The bootstrap/config.go file defines available keys.

6. **Error handling:** Create templates in `internal/app/features/errors/` for custom error pages. The errors feature is mounted in the router.

7. **Documentation in docs/:** API docs, database schema, CSRF implementation, webhook system, UI design patterns are documented. Check there before asking questions.

8. **Test utilities:** Use `internal/testutil/` for test helpers (test database setup, fixtures, etc.).

9. **Dependency updates:** Run `go mod tidy` after adding/removing dependencies. The project uses Go 1.24.1, verify compatibility.

10. **HTML Sanitization:** User-submitted HTML is sanitized with `bluemonday` before rendering. Look for `.Safe()` template function usage.

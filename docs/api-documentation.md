# API Documentation System Design

This document describes the design and implementation plan for adding OpenAPI/Swagger API documentation to Strata.

## Overview

An API documentation system provides machine-readable specifications and interactive documentation for REST APIs. This enables client SDK generation, request validation, and developer-friendly API exploration.

### Goals

- Provide interactive API documentation via Swagger UI
- Generate OpenAPI 3.0 specification from code
- Enable automatic request/response validation
- Support client SDK generation for multiple languages
- Keep documentation in sync with implementation

### Non-Goals (Initial Release)

- GraphQL documentation
- gRPC/protobuf documentation
- API versioning strategy (v1, v2, etc.)
- Public API rate limiting dashboard

---

## Technology Choice

### Recommended: swaggo/swag (Code-First)

After evaluating options, **swaggo/swag** is recommended for Strata because:

1. **Chi-compatible** - Works with existing router
2. **Comment-based** - Non-invasive, uses Go comments
3. **Popular** - Large community, well-maintained
4. **Simple** - Minimal setup, easy to adopt incrementally

| Library | Approach | Chi Support | Complexity |
|---------|----------|-------------|------------|
| swaggo/swag | Code-first | Yes | Low |
| go-swagger | Code-first | Yes | Medium |
| oapi-codegen | Spec-first | Yes | Medium |
| huma | Runtime | No (own router) | High |

### Dependencies

```go
// go.mod additions
require (
    github.com/swaggo/swag v1.16.0
    github.com/swaggo/http-swagger v1.3.0
)
```

### CLI Tool

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

---

## OpenAPI Specification

### API Info

```yaml
openapi: 3.0.0
info:
  title: Strata API
  description: REST API for Strata application
  version: 1.0.0
  contact:
    name: API Support
    email: support@example.com
servers:
  - url: https://api.example.com
    description: Production
  - url: http://localhost:8080
    description: Development
```

### Authentication

```yaml
securityDefinitions:
  BearerAuth:
    type: apiKey
    in: header
    name: Authorization
    description: "Enter: Bearer {your_api_key}"
```

### Common Response Schemas

```yaml
components:
  schemas:
    Error:
      type: object
      properties:
        error:
          type: string
          example: "Invalid request"
        code:
          type: string
          example: "INVALID_INPUT"

    Pagination:
      type: object
      properties:
        page:
          type: integer
          example: 1
        per_page:
          type: integer
          example: 20
        total:
          type: integer
          example: 100
        total_pages:
          type: integer
          example: 5
```

---

## API Endpoints

### Planned API Structure

```
/api/v1/
├── /users
│   ├── GET    /           - List users
│   ├── POST   /           - Create user
│   ├── GET    /{id}       - Get user
│   ├── PUT    /{id}       - Update user
│   └── DELETE /{id}       - Delete user
│
├── /invitations
│   ├── GET    /           - List invitations
│   ├── POST   /           - Create invitation
│   ├── DELETE /{id}       - Revoke invitation
│   └── POST   /{id}/resend - Resend invitation
│
├── /announcements
│   ├── GET    /           - List announcements
│   ├── POST   /           - Create announcement
│   ├── GET    /{id}       - Get announcement
│   ├── PUT    /{id}       - Update announcement
│   └── DELETE /{id}       - Delete announcement
│
├── /settings
│   ├── GET    /           - Get settings
│   └── PUT    /           - Update settings
│
├── /audit
│   └── GET    /           - List audit events
│
├── /sessions
│   ├── GET    /           - List active sessions
│   └── DELETE /{id}       - Terminate session
│
└── /webhooks (if implemented)
    ├── GET    /           - List webhooks
    ├── POST   /           - Create webhook
    ├── GET    /{id}       - Get webhook
    ├── PUT    /{id}       - Update webhook
    ├── DELETE /{id}       - Delete webhook
    └── POST   /{id}/test  - Test webhook
```

### Example Endpoint Documentation

```go
// ListUsers godoc
// @Summary      List users
// @Description  Get a paginated list of users with optional filters
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query    int     false  "Page number"           default(1)
// @Param        per_page  query    int     false  "Items per page"        default(20)  maximum(100)
// @Param        status    query    string  false  "Filter by status"      Enums(active, disabled)
// @Param        role      query    string  false  "Filter by role"        Enums(admin, user)
// @Param        search    query    string  false  "Search by name/email"
// @Success      200  {object}  UsersListResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/users [get]
func (h *APIHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

---

## Project Structure

### New Files

```
stratasave/
├── api/
│   └── swagger.json              # Generated OpenAPI spec (gitignored)
├── cmd/stratasave/
│   └── main.go                   # Add swag init in build
├── docs/
│   └── api-documentation.md      # This document
├── internal/app/
│   ├── api/                      # New: API handlers package
│   │   ├── doc.go                # Swag main annotations
│   │   ├── types.go              # Request/response types
│   │   ├── users.go              # User API handlers
│   │   ├── invitations.go        # Invitation API handlers
│   │   ├── announcements.go      # Announcement API handlers
│   │   ├── settings.go           # Settings API handlers
│   │   ├── audit.go              # Audit API handlers
│   │   ├── sessions.go           # Session API handlers
│   │   └── webhooks.go           # Webhook API handlers (if implemented)
│   └── bootstrap/
│       └── routes.go             # Mount API routes and Swagger UI
└── Makefile                      # Add swag generate target
```

### Main Swagger Annotations (doc.go)

```go
// Package api Strata API
//
// REST API for managing users, invitations, announcements, and settings.
//
//     Schemes: https, http
//     Host: localhost:8080
//     BasePath: /api/v1
//     Version: 1.0.0
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//     Security:
//     - BearerAuth: []
//
//     SecurityDefinitions:
//       BearerAuth:
//         type: apiKey
//         in: header
//         name: Authorization
//
// swagger:meta
package api
```

---

## Request/Response Types

### Common Types

```go
// internal/app/api/types.go

// ErrorResponse represents an API error
// swagger:model
type ErrorResponse struct {
    // Error message
    // example: Invalid request parameters
    Error string `json:"error"`

    // Error code for programmatic handling
    // example: INVALID_INPUT
    Code string `json:"code,omitempty"`

    // Field-specific errors
    // example: {"email": "invalid format"}
    Details map[string]string `json:"details,omitempty"`
}

// PaginationMeta contains pagination information
// swagger:model
type PaginationMeta struct {
    // Current page number
    // example: 1
    Page int `json:"page"`

    // Items per page
    // example: 20
    PerPage int `json:"per_page"`

    // Total number of items
    // example: 100
    Total int64 `json:"total"`

    // Total number of pages
    // example: 5
    TotalPages int `json:"total_pages"`
}

// SuccessResponse for operations without data
// swagger:model
type SuccessResponse struct {
    // Success message
    // example: Operation completed successfully
    Message string `json:"message"`
}
```

### User Types

```go
// UserResponse represents a user in API responses
// swagger:model
type UserResponse struct {
    // User ID
    // example: 507f1f77bcf86cd799439011
    ID string `json:"id"`

    // Full name
    // example: John Doe
    FullName string `json:"full_name"`

    // Email address
    // example: john@example.com
    Email string `json:"email,omitempty"`

    // User role
    // example: admin
    // enum: admin,user
    Role string `json:"role"`

    // Account status
    // example: active
    // enum: active,disabled
    Status string `json:"status"`

    // Authentication method
    // example: email
    // enum: trust,password,email,google
    AuthMethod string `json:"auth_method"`

    // Creation timestamp
    // example: 2026-01-19T15:30:00Z
    CreatedAt string `json:"created_at"`
}

// UsersListResponse is the response for listing users
// swagger:model
type UsersListResponse struct {
    // List of users
    Users []UserResponse `json:"users"`

    // Pagination metadata
    Meta PaginationMeta `json:"meta"`
}

// CreateUserRequest is the request body for creating a user
// swagger:model
type CreateUserRequest struct {
    // Full name (required)
    // example: John Doe
    // required: true
    FullName string `json:"full_name" validate:"required"`

    // Email address (required for email auth)
    // example: john@example.com
    Email string `json:"email" validate:"omitempty,email"`

    // Login ID (required for password/trust auth)
    // example: johndoe
    LoginID string `json:"login_id"`

    // Authentication method
    // example: email
    // enum: trust,password,email,google
    // required: true
    AuthMethod string `json:"auth_method" validate:"required,oneof=trust password email google"`

    // User role
    // example: admin
    // enum: admin
    // required: true
    Role string `json:"role" validate:"required,oneof=admin"`

    // Temporary password (required for password auth)
    // example: TempPass123!
    Password string `json:"password,omitempty"`
}

// UpdateUserRequest is the request body for updating a user
// swagger:model
type UpdateUserRequest struct {
    // Full name
    // example: John Doe
    FullName string `json:"full_name,omitempty"`

    // Email address
    // example: john@example.com
    Email string `json:"email,omitempty" validate:"omitempty,email"`

    // Account status
    // example: active
    // enum: active,disabled
    Status string `json:"status,omitempty" validate:"omitempty,oneof=active disabled"`
}
```

---

## Swagger UI Integration

### Route Configuration

```go
// internal/app/bootstrap/routes.go

import (
    httpSwagger "github.com/swaggo/http-swagger"
    _ "github.com/dalemusser/stratasave/api" // Import generated docs
)

func BuildHandler(...) (http.Handler, error) {
    r := chi.NewRouter()

    // ... existing middleware ...

    // Swagger UI (only in dev or if explicitly enabled)
    if coreCfg.Env == "dev" || appCfg.SwaggerEnabled {
        r.Get("/swagger/*", httpSwagger.Handler(
            httpSwagger.URL("/swagger/doc.json"),
            httpSwagger.DeepLinking(true),
            httpSwagger.DocExpansion("list"),
            httpSwagger.DomID("swagger-ui"),
        ))
        logger.Info("Swagger UI enabled", zap.String("url", "/swagger/index.html"))
    }

    // API v1 routes
    if appCfg.APIKey != "" {
        r.Route("/api/v1", func(r chi.Router) {
            r.Use(apicors.Middleware())
            r.Use(auth.APIKeyAuth(appCfg.APIKey, logger))

            apiHandler := api.NewHandler(deps, logger)
            r.Mount("/users", apiHandler.UserRoutes())
            r.Mount("/invitations", apiHandler.InvitationRoutes())
            r.Mount("/announcements", apiHandler.AnnouncementRoutes())
            r.Mount("/settings", apiHandler.SettingsRoutes())
            r.Mount("/audit", apiHandler.AuditRoutes())
            r.Mount("/sessions", apiHandler.SessionRoutes())
        })
    }

    // ... rest of routes ...
}
```

### Swagger UI Customization

```go
httpSwagger.Handler(
    httpSwagger.URL("/swagger/doc.json"),

    // UI behavior
    httpSwagger.DeepLinking(true),
    httpSwagger.DocExpansion("list"),        // none, list, full
    httpSwagger.DefaultModelsExpandDepth(1),

    // Branding
    httpSwagger.UIConfig(map[string]string{
        "displayRequestDuration": "true",
        "filter":                 "true",
    }),
)
```

---

## Build Process

### Makefile Targets

```makefile
# Generate Swagger documentation
.PHONY: swagger
swagger:
	@echo "Generating Swagger documentation..."
	swag init -g internal/app/api/doc.go -o api --parseDependency --parseInternal

# Generate and format
.PHONY: swagger-fmt
swagger-fmt: swagger
	swag fmt -g internal/app/api/doc.go

# Build with Swagger generation
.PHONY: build
build: swagger
	go build -o bin/stratasave ./cmd/stratasave

# Development: regenerate on file changes
.PHONY: swagger-watch
swagger-watch:
	@echo "Watching for changes..."
	fswatch -o internal/app/api/*.go | xargs -n1 -I{} make swagger
```

### CI/CD Integration

```yaml
# .github/workflows/build.yml
jobs:
  build:
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Install swag
        run: go install github.com/swaggo/swag/cmd/swag@latest

      - name: Generate Swagger docs
        run: make swagger

      - name: Verify Swagger docs are up to date
        run: |
          git diff --exit-code api/swagger.json || \
            (echo "Swagger docs out of date. Run 'make swagger' and commit." && exit 1)

      - name: Build
        run: go build ./...
```

---

## Configuration

### New Configuration Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `swagger_enabled` | bool | `false` | Enable Swagger UI in production |
| `api_title` | string | `"Strata API"` | API title in Swagger UI |
| `api_version` | string | `"1.0.0"` | API version in Swagger UI |

### AppConfig Updates

```go
// internal/app/bootstrap/appconfig.go

type AppConfig struct {
    // ... existing fields ...

    // API Documentation
    SwaggerEnabled bool   `toml:"swagger_enabled" env:"SWAGGER_ENABLED"`
    APITitle       string `toml:"api_title" env:"API_TITLE"`
    APIVersion     string `toml:"api_version" env:"API_VERSION"`
}
```

---

## Security Considerations

### Swagger UI Access

1. **Development**: Swagger UI enabled by default
2. **Production**: Disabled by default, enable with `swagger_enabled = true`
3. **Authentication**: Consider adding basic auth to Swagger UI in production

```go
// Optional: Protect Swagger UI with basic auth
if appCfg.SwaggerEnabled && appCfg.SwaggerPassword != "" {
    r.Group(func(r chi.Router) {
        r.Use(middleware.BasicAuth("Swagger", map[string]string{
            appCfg.SwaggerUser: appCfg.SwaggerPassword,
        }))
        r.Get("/swagger/*", httpSwagger.Handler(...))
    })
}
```

### API Security Headers

```go
// Add security headers to API responses
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Cache-Control", "no-store")
        next.ServeHTTP(w, r)
    })
}
```

### Sensitive Data

- Never expose sensitive fields in API responses (passwords, secrets)
- Use `json:"-"` tag to exclude fields from serialization
- Document which fields are returned vs. accepted

---

## Implementation Plan

### Phase 1: Infrastructure Setup

**Estimated scope:** Foundation for API documentation

1. Add swaggo dependencies to `go.mod`
2. Create `internal/app/api/` package structure
3. Create `doc.go` with main Swagger annotations
4. Create `types.go` with common request/response types
5. Add Makefile targets for Swagger generation
6. Mount Swagger UI in `routes.go`

**Files to create:**
- `internal/app/api/doc.go`
- `internal/app/api/types.go`
- `internal/app/api/handler.go`
- `api/.gitkeep` (generated docs directory)

**Files to modify:**
- `go.mod`
- `Makefile`
- `internal/app/bootstrap/routes.go`
- `internal/app/bootstrap/appconfig.go`
- `.gitignore` (add `api/swagger.*`)

### Phase 2: User API

**Estimated scope:** Complete user management API

1. Create `internal/app/api/users.go` with annotated handlers
2. Implement CRUD operations for users
3. Add request validation
4. Add pagination support
5. Write tests

**Endpoints:**
- `GET /api/v1/users` - List users
- `POST /api/v1/users` - Create user
- `GET /api/v1/users/{id}` - Get user
- `PUT /api/v1/users/{id}` - Update user
- `DELETE /api/v1/users/{id}` - Delete user
- `POST /api/v1/users/{id}/disable` - Disable user
- `POST /api/v1/users/{id}/enable` - Enable user

### Phase 3: Invitation API

**Estimated scope:** Invitation management API

1. Create `internal/app/api/invitations.go`
2. Implement invitation operations
3. Document request/response types

**Endpoints:**
- `GET /api/v1/invitations` - List pending invitations
- `POST /api/v1/invitations` - Create invitation
- `DELETE /api/v1/invitations/{id}` - Revoke invitation
- `POST /api/v1/invitations/{id}/resend` - Resend invitation

### Phase 4: Settings & Announcements API

**Estimated scope:** Settings and announcements API

1. Create `internal/app/api/settings.go`
2. Create `internal/app/api/announcements.go`
3. Implement CRUD operations

**Endpoints:**
- `GET /api/v1/settings` - Get site settings
- `PUT /api/v1/settings` - Update site settings
- `GET /api/v1/announcements` - List announcements
- `POST /api/v1/announcements` - Create announcement
- `GET /api/v1/announcements/{id}` - Get announcement
- `PUT /api/v1/announcements/{id}` - Update announcement
- `DELETE /api/v1/announcements/{id}` - Delete announcement

### Phase 5: Audit & Sessions API

**Estimated scope:** Read-only audit and session APIs

1. Create `internal/app/api/audit.go`
2. Create `internal/app/api/sessions.go`
3. Implement list/read operations

**Endpoints:**
- `GET /api/v1/audit` - List audit events (with filters)
- `GET /api/v1/sessions` - List active sessions
- `DELETE /api/v1/sessions/{id}` - Terminate session

### Phase 6: Documentation & Polish

**Estimated scope:** Final documentation and testing

1. Add API usage examples to documentation
2. Create Postman/Insomnia collection
3. Add integration tests for all endpoints
4. Update `docs/configuration.md`
5. Create API changelog

---

## Example API Usage

### Authentication

All API requests require a Bearer token:

```bash
curl -X GET "https://api.example.com/api/v1/users" \
  -H "Authorization: Bearer your-api-key-here"
```

### List Users

```bash
curl -X GET "https://api.example.com/api/v1/users?page=1&per_page=10&status=active" \
  -H "Authorization: Bearer your-api-key-here"
```

**Response:**
```json
{
  "users": [
    {
      "id": "507f1f77bcf86cd799439011",
      "full_name": "John Doe",
      "email": "john@example.com",
      "role": "admin",
      "status": "active",
      "auth_method": "email",
      "created_at": "2026-01-19T15:30:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 10,
    "total": 1,
    "total_pages": 1
  }
}
```

### Create User

```bash
curl -X POST "https://api.example.com/api/v1/users" \
  -H "Authorization: Bearer your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "full_name": "Jane Smith",
    "email": "jane@example.com",
    "auth_method": "email",
    "role": "admin"
  }'
```

**Response:**
```json
{
  "id": "507f1f77bcf86cd799439012",
  "full_name": "Jane Smith",
  "email": "jane@example.com",
  "role": "admin",
  "status": "active",
  "auth_method": "email",
  "created_at": "2026-01-19T16:00:00Z"
}
```

### Error Response

```json
{
  "error": "Validation failed",
  "code": "VALIDATION_ERROR",
  "details": {
    "email": "invalid email format",
    "auth_method": "must be one of: trust, password, email, google"
  }
}
```

---

## Future Enhancements

These are not included in the initial implementation but may be added later:

1. **API Versioning** - Support multiple API versions (v1, v2)
2. **Rate Limiting Dashboard** - Show rate limit status in responses
3. **API Key Management** - Multiple API keys with different permissions
4. **Webhook API** - If webhook system is implemented
5. **SDK Generation** - Auto-generate client libraries
6. **API Analytics** - Track API usage and performance
7. **OpenAPI Validation Middleware** - Validate requests against spec
8. **Redoc Alternative** - Offer Redoc as alternative to Swagger UI

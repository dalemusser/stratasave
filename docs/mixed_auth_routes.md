# Mixed Authentication Routes

This document describes how to set up routes with different authentication strategies in StrataSave-based applications.

---

## Overview

Some applications need two authentication modes:

| Route Group | Auth Method | Use Case |
|-------------|-------------|----------|
| `/api/*` | API Key (Bearer token) | External consumers (games, mobile apps, integrations) |
| Web UI routes | Session cookies + CSRF | Browser-based admin/dashboard |

This is common for applications that:
- Provide a developer dashboard (session auth)
- Accept data from external services (API key auth)
- Expose REST APIs to third-party integrations

---

## Route Configuration Pattern

```go
// internal/app/bootstrap/routes.go

import (
    "github.com/dalemusser/stratasave/internal/app/system/apicors"
    "github.com/dalemusser/stratasave/internal/app/system/auth"
    // ... other imports
)

func BuildHandler(coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) (http.Handler, error) {
    r := chi.NewRouter()

    // ─────────────────────────────────────────────────────────────────────────
    // API Routes - API Key Auth, No CSRF, Permissive CORS
    // ─────────────────────────────────────────────────────────────────────────
    //
    // These routes are for external API consumers (games, mobile apps, etc.)
    // that authenticate via API key rather than session cookies.
    //
    // Key differences from web UI routes:
    //   - API key auth (Bearer token) instead of session cookies
    //   - No CSRF protection (not needed for API key auth)
    //   - Permissive CORS (AllowCredentials: false, Origin: *)
    //
    if appCfg.APIKey != "" {
        r.Group(func(r chi.Router) {
            r.Use(apicors.Middleware())                    // Allow any origin, no credentials
            r.Use(auth.APIKeyAuth(appCfg.APIKey, logger))  // Validate Bearer token
            // Note: No CSRF middleware - API key auth is not CSRF-vulnerable
            // Note: No session middleware - API requests don't use sessions

            r.Mount("/api", apiRoutes(deps, logger))
        })
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Web UI Routes - Session Auth, CSRF Protection
    // ─────────────────────────────────────────────────────────────────────────
    //
    // These routes are for browser-based access with session cookies.
    //
    r.Group(func(r chi.Router) {
        r.Use(sessionMgr.LoadSessionUser)
        r.Use(csrfMiddleware)

        // Public pages
        r.Mount("/", homeRoutes)
        r.Mount("/login", loginRoutes)

        // Protected pages (require login)
        r.Group(func(r chi.Router) {
            r.Use(sessionMgr.RequireAuth)
            r.Mount("/dashboard", dashboardRoutes)
        })

        // Admin pages (require admin role)
        r.Group(func(r chi.Router) {
            r.Use(sessionMgr.RequireRole("admin"))
            r.Mount("/settings", settingsRoutes)
        })
    })

    return r, nil
}
```

---

## Why Different CORS for API Routes?

### Web UI Routes (Session Auth)
- Use restrictive CORS with specific allowed origins
- `AllowCredentials: true` to send cookies
- `SameSite=None` + `Secure=true` for cross-subdomain cookies

### API Routes (API Key Auth)
- Use permissive CORS: `Origin: *`
- `AllowCredentials: false` (no cookies needed)
- Any origin can make requests, but must provide valid API key

The API key itself provides authentication, so there's no need to restrict origins. Cookies aren't involved, so CSRF isn't a concern.

---

## Why No CSRF for API Routes?

CSRF attacks exploit the browser's automatic cookie sending. They trick a user's browser into making authenticated requests to a site where the user is logged in.

API key authentication is not vulnerable to CSRF because:
1. The API key is not automatically sent by browsers (unlike cookies)
2. The client must explicitly include the `Authorization` header
3. Cross-origin requests cannot read the API key from another page

---

## Helper Packages

### `auth.APIKeyAuth(key, logger)`
Middleware that validates Bearer token authentication.

```go
r.Use(auth.APIKeyAuth(appCfg.APIKey, logger))
```

### `apicors.Middleware()`
CORS middleware for API routes (allows any origin, no credentials).

```go
r.Use(apicors.Middleware())
```

### `apicors.MiddlewareWithOrigins(origins...)`
CORS middleware that restricts to specific origins.

```go
r.Use(apicors.MiddlewareWithOrigins("https://game1.example.com", "https://game2.example.com"))
```

### `jsonutil` Package
Helper functions for JSON API responses.

```go
import "github.com/dalemusser/stratasave/internal/app/system/jsonutil"

// Success responses
jsonutil.OK(w, data)
jsonutil.Created(w, data)
jsonutil.NoContent(w)

// Error responses
jsonutil.BadRequest(w, "invalid input")
jsonutil.Unauthorized(w, "missing API key")
jsonutil.NotFound(w, "resource not found")
jsonutil.InternalError(w, "something went wrong")

// Decode request body
var input MyInput
if err := jsonutil.Decode(r, &input); err != nil {
    jsonutil.BadRequest(w, "invalid JSON")
    return
}
```

---

## Configuration

Add the API key to your environment:

```bash
# .env or environment
STRATASAVE_API_KEY=your-secret-api-key-here
```

Or for derived apps (replace STRATA with your app prefix):
```bash
MYAPP_API_KEY=your-secret-api-key-here
```

The API key should be:
- At least 32 characters
- Randomly generated
- Kept secret and rotated periodically

Generate a secure key:
```bash
openssl rand -base64 32
```

---

## Client Usage

External clients authenticate by including the API key in the Authorization header:

```bash
curl -X POST https://api.example.com/api/logs \
  -H "Authorization: Bearer your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{"event": "test"}'
```

JavaScript example:
```javascript
fetch('https://api.example.com/api/logs', {
    method: 'POST',
    headers: {
        'Authorization': 'Bearer your-api-key-here',
        'Content-Type': 'application/json'
    },
    body: JSON.stringify({ event: 'test' })
});
```

---

## Security Considerations

1. **API Key Storage**: Store the API key securely in environment variables, never in code
2. **HTTPS Only**: Always use HTTPS in production to protect the API key in transit
3. **Key Rotation**: Rotate API keys periodically and when compromised
4. **Logging**: Log authentication failures but never log the actual API key
5. **Rate Limiting**: Consider adding rate limiting to API endpoints

// Package apicors provides CORS middleware for API endpoints that use
// API key authentication instead of cookies.
//
// When using API key authentication:
//   - Credentials (cookies) are not needed, so AllowCredentials can be false
//   - Origins can be "*" (any origin) since there are no cookies to protect
//   - This is more permissive than session-based auth CORS
//
// This is the CORS configuration pattern for external API consumers
// (games, mobile apps, third-party integrations) that authenticate via API key.
package apicors

import (
	"net/http"
)

// Middleware returns CORS middleware suitable for API key authenticated endpoints.
//
// This middleware:
//   - Allows any origin (Access-Control-Allow-Origin: *)
//   - Does not allow credentials (no cookies needed with API key auth)
//   - Allows common API methods and headers
//   - Handles preflight OPTIONS requests
//
// Usage in routes.go:
//
//	// API routes - API key auth, permissive CORS, no CSRF
//	r.Group(func(r chi.Router) {
//	    r.Use(apicors.Middleware())
//	    r.Use(auth.APIKeyAuth(appCfg.APIKey, logger))
//	    // Note: No CSRF middleware here - API key auth is not CSRF-vulnerable
//	    r.Mount("/api", apiRoutes)
//	})
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers for API access
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
			w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

			// Handle preflight OPTIONS request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareWithOrigins returns CORS middleware that only allows specific origins.
// Use this when you want API key auth but still want to restrict which domains
// can make requests.
//
// Usage:
//
//	r.Use(apicors.MiddlewareWithOrigins("https://game1.example.com", "https://game2.example.com"))
func MiddlewareWithOrigins(allowedOrigins ...string) func(http.Handler) http.Handler {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if origin != "" {
				if _, allowed := originSet[origin]; allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
				// If origin not allowed, don't set CORS headers (browser will block)
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

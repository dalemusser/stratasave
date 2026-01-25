package auth

import (
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// APIKeyAuth returns middleware that validates API key authentication.
//
// The middleware checks for an API key in the Authorization header using
// the Bearer scheme: "Authorization: Bearer <api-key>".
//
// Parameters:
//   - validKey: the expected API key (from configuration)
//   - logger: for logging authentication failures
//
// Usage in routes.go:
//
//	// API routes - API key auth, no CSRF, permissive CORS
//	r.Group(func(r chi.Router) {
//	    r.Use(apicors.Middleware())  // Allow any origin, no credentials
//	    r.Use(auth.APIKeyAuth(appCfg.APIKey, logger))
//	    r.Mount("/api", apiRoutes)
//	})
//
// If the API key is invalid or missing, returns 401 Unauthorized.
// If the API key is not configured (empty), logs a warning and rejects all requests.
func APIKeyAuth(validKey string, logger *zap.Logger) func(http.Handler) http.Handler {
	if validKey == "" {
		logger.Warn("API key not configured - all API requests will be rejected")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no API key is configured, reject all requests
			if validKey == "" {
				logger.Warn("API request rejected: API key not configured",
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, "API authentication not configured", http.StatusUnauthorized)
				return
			}

			// Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Debug("API request rejected: missing Authorization header",
					zap.String("path", r.URL.Path),
				)
				http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
				return
			}

			// Expect "Bearer <api-key>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				logger.Debug("API request rejected: invalid Authorization format",
					zap.String("path", r.URL.Path),
				)
				http.Error(w, "Invalid Authorization format (expected: Bearer <api-key>)", http.StatusUnauthorized)
				return
			}

			providedKey := parts[1]
			if providedKey != validKey {
				logger.Warn("API request rejected: invalid API key",
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Valid API key - proceed
			next.ServeHTTP(w, r)
		})
	}
}

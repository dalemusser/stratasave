package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// APIKeys builds the list of accepted API keys from the primary key and a
// comma-separated list of extra keys (config keys api_key and
// api_keys_extra). Blank entries and repeats are dropped; the primary key,
// when set, comes first.
func APIKeys(primary, extra string) []string {
	var keys []string
	seen := map[string]struct{}{}
	for _, k := range append([]string{primary}, strings.Split(extra, ",")...) {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	return keys
}

// APIKeyAuth returns middleware that validates API key authentication.
//
// The middleware checks for an API key in the Authorization header using
// the Bearer scheme: "Authorization: Bearer <api-key>".
//
// Parameters:
//   - validKeys: every key that is accepted (from configuration; see APIKeys).
//     More than one key lets a key be replaced without downtime: add the new
//     key, move the clients to it, then remove the old one.
//   - logger: for logging authentication failures
//
// Usage in routes.go:
//
//	// API routes - API key auth, no CSRF, permissive CORS
//	r.Group(func(r chi.Router) {
//	    r.Use(apicors.Middleware())  // Allow any origin, no credentials
//	    r.Use(auth.APIKeyAuth(appCfg.APIKeys, logger))
//	    r.Mount("/api", apiRoutes)
//	})
//
// If the API key is invalid or missing, returns 401 Unauthorized.
// If no API key is configured, logs a warning and rejects all requests.
func APIKeyAuth(validKeys []string, logger *zap.Logger) func(http.Handler) http.Handler {
	keys := make([][]byte, 0, len(validKeys))
	for _, k := range validKeys {
		if k != "" {
			keys = append(keys, []byte(k))
		}
	}
	if len(keys) == 0 {
		logger.Warn("API key not configured - all API requests will be rejected")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no API key is configured, reject all requests
			if len(keys) == 0 {
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

			// Compare against every key, without stopping at the first match,
			// so the time taken says nothing about which key matched.
			providedKey := []byte(parts[1])
			matched := 0
			for _, k := range keys {
				matched |= subtle.ConstantTimeCompare(providedKey, k)
			}
			if matched != 1 {
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

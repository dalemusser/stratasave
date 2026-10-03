package auth

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
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

// RestrictedKey is an API key that is accepted only for requests about
// particular users (config keys api_key_restricted, api_key_restricted_user_ids
// and api_key_restricted_enforce). It is for a key that cannot be kept secret,
// such as one built into a game's source for running it without a host: such
// a key may read and write the data of the game's built-in test user and
// nobody else's.
type RestrictedKey struct {
	// Key is the key itself; empty means there is no restricted key.
	Key string
	// UserIDs are the user ids the key may be used for.
	UserIDs []string
	// Enforce refuses a request about any other user (403). When false the
	// request is let through and logged, which shows who still uses the key
	// before refusing starts.
	Enforce bool
}

// SplitList splits a comma-separated config value, dropping blanks.
func SplitList(value string) []string {
	var out []string
	for _, v := range strings.Split(value, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// The largest request body the restricted-key check reads. The API handlers
// accept less than this.
const restrictedMaxBody = 2 << 20

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
	return APIKeyAuthRestricted(validKeys, RestrictedKey{}, logger)
}

// APIKeyAuthRestricted is APIKeyAuth with a restricted key as well: a request
// that presents restricted.Key is accepted only when every user id it names
// is one of restricted.UserIDs (403 otherwise; see RestrictedKey.Enforce).
func APIKeyAuthRestricted(validKeys []string, restricted RestrictedKey, logger *zap.Logger) func(http.Handler) http.Handler {
	keys := make([][]byte, 0, len(validKeys))
	for _, k := range validKeys {
		if k != "" {
			keys = append(keys, []byte(k))
		}
	}
	restrictedKey := []byte(restricted.Key)
	allowedUsers := make(map[string]struct{}, len(restricted.UserIDs))
	for _, id := range restricted.UserIDs {
		allowedUsers[id] = struct{}{}
	}
	if len(keys) == 0 && len(restrictedKey) == 0 {
		logger.Warn("API key not configured - all API requests will be rejected")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no API key is configured, reject all requests
			if len(keys) == 0 && len(restrictedKey) == 0 {
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
			restrictedMatch := 0
			if len(restrictedKey) > 0 {
				restrictedMatch = subtle.ConstantTimeCompare(providedKey, restrictedKey)
			}

			if matched == 1 {
				// Valid API key - proceed
				next.ServeHTTP(w, r)
				return
			}

			if restrictedMatch == 1 {
				ids, ok := requestUserIDs(r)
				allowed := ok && len(ids) > 0
				for _, id := range ids {
					if _, in := allowedUsers[id]; !in {
						allowed = false
					}
				}
				if allowed {
					next.ServeHTTP(w, r)
					return
				}
				logger.Warn("restricted API key used outside its user ids",
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method),
					zap.String("remote_addr", r.RemoteAddr),
					zap.Strings("user_ids", ids),
					zap.Bool("refused", restricted.Enforce),
				)
				if !restricted.Enforce {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "This API key is not accepted for this user_id", http.StatusForbidden)
				return
			}

			logger.Warn("API request rejected: invalid API key",
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
		})
	}
}

// requestUserIDs returns every user id a request names, so a restricted key
// can be held to its users. ok is false when the ids cannot be established
// (an unreadable or oversized body, a body that is not a JSON object, a user
// id that is not a string); the caller treats that as not allowed.
//
// A request with a body is read as the handlers read it: the first JSON
// value. Every key that equals "user_id" in any letter case counts, at the
// top level and in each element of a batch's "entries", so the result
// covers whichever of them a handler ends up using (the handlers match the
// key exactly or, when decoding into a struct, without regard to case). The
// body is put back for the handler.
//
// A request without a body (the list endpoint) names its user in the
// "user_id" query parameter; without one it is about every user.
func requestUserIDs(r *http.Request) (ids []string, ok bool) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return r.URL.Query()["user_id"], true
	}
	if r.Body == nil {
		return nil, true
	}
	// Read up to the limit and hand the handler the same bytes followed by
	// whatever was not read, so a request that is let through is unchanged.
	orig := r.Body
	body, err := io.ReadAll(io.LimitReader(orig, restrictedMaxBody+1))
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(body), orig), orig}
	if err != nil || len(body) > restrictedMaxBody {
		return nil, false
	}

	var top map[string]json.RawMessage
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&top); err != nil || top == nil {
		return nil, false
	}
	collect := func(obj map[string]json.RawMessage) bool {
		for k, v := range obj {
			if !strings.EqualFold(k, "user_id") {
				continue
			}
			var id string
			if err := json.Unmarshal(v, &id); err != nil {
				return false
			}
			ids = append(ids, id)
		}
		return true
	}
	if !collect(top) {
		return ids, false
	}
	for k, v := range top {
		if !strings.EqualFold(k, "entries") {
			continue
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(v, &entries); err != nil {
			continue // not a list: the handler treats the request as a single entry
		}
		for _, e := range entries {
			var entry map[string]json.RawMessage
			if err := json.Unmarshal(e, &entry); err != nil || entry == nil {
				return ids, false
			}
			if !collect(entry) {
				return ids, false
			}
		}
	}
	return ids, true
}

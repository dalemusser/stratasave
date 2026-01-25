package testutil

import (
	"context"
	"net/http"
)

// csrfTokenKey matches the key used by gorilla/csrf internally.
// This allows us to inject a mock token for testing.
const csrfTokenKey = "gorilla.csrf.Token"

// WithCSRFToken adds a mock CSRF token to the request context.
// This prevents panics or empty tokens when handlers call csrf.Token(r)
// or use viewdata.NewBaseVM which calls csrf.Token internally.
//
// Usage:
//
//	req := httptest.NewRequest(http.MethodGet, "/path", nil)
//	req = testutil.WithCSRFToken(req)
//	handler.ServeHTTP(rec, req)
func WithCSRFToken(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), csrfTokenKey, "test-csrf-token-12345")
	return r.WithContext(ctx)
}

// NewAuthenticatedRequestWithCSRF creates an HTTP request with both a user
// and CSRF token in context. This is the recommended way to create requests
// for testing handlers that render forms.
func NewAuthenticatedRequestWithCSRF(method, target string, user TestUser) *http.Request {
	req := NewAuthenticatedRequest(method, target, user)
	return WithCSRFToken(req)
}

package testutil

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestUser represents user data for testing HTTP handlers.
type TestUser struct {
	ID    string
	Name  string
	Email string
	Role  string
}

// AdminUser returns a TestUser with admin role.
func AdminUser() TestUser {
	return TestUser{
		ID:    primitive.NewObjectID().Hex(),
		Name:  "Test Admin",
		Email: "admin@test.com",
		Role:  "admin",
	}
}

// WithUser adds a user to the request context for testing authenticated handlers.
// This bypasses the session middleware and injects the user directly.
func WithUser(r *http.Request, user TestUser) *http.Request {
	sessionUser := &auth.SessionUser{
		ID:      user.ID,
		Name:    user.Name,
		LoginID: user.Email,
		Role:    user.Role,
	}
	return auth.WithTestUser(r, sessionUser)
}

// NewRequest creates an HTTP request for testing.
func NewRequest(method, target string) *http.Request {
	return httptest.NewRequest(method, target, nil)
}

// NewAuthenticatedRequest creates an HTTP request with a user in context.
func NewAuthenticatedRequest(method, target string, user TestUser) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	return WithUser(req, user)
}

// ResponseRecorder wraps httptest.ResponseRecorder with helper methods.
type ResponseRecorder struct {
	*httptest.ResponseRecorder
}

// NewRecorder creates a new ResponseRecorder.
func NewRecorder() *ResponseRecorder {
	return &ResponseRecorder{httptest.NewRecorder()}
}

// AssertStatus checks the response status code.
func (r *ResponseRecorder) AssertStatus(t interface{ Errorf(string, ...any) }, expected int) {
	if r.Code != expected {
		t.Errorf("status code: got %d, want %d", r.Code, expected)
	}
}

// AssertRedirect checks for a redirect to the expected location.
func (r *ResponseRecorder) AssertRedirect(t interface{ Errorf(string, ...any) }, expectedLocation string) {
	if r.Code != http.StatusSeeOther && r.Code != http.StatusFound && r.Code != http.StatusMovedPermanently {
		t.Errorf("expected redirect status, got %d", r.Code)
	}
	location := r.Header().Get("Location")
	if location != expectedLocation {
		t.Errorf("redirect location: got %q, want %q", location, expectedLocation)
	}
}

// AssertContains checks if the response body contains the expected string.
func (r *ResponseRecorder) AssertContains(t interface{ Errorf(string, ...any) }, expected string) {
	body := r.Body.String()
	if !strings.Contains(body, expected) {
		t.Errorf("response body does not contain %q", expected)
	}
}

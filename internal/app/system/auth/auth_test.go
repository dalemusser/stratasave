package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

func TestNewSessionManager(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name       string
		sessionKey string
		secure     bool
		wantErr    bool
	}{
		{
			name:       "valid key dev mode",
			sessionKey: "this-is-a-32-character-long-key!",
			secure:     false,
			wantErr:    false,
		},
		{
			name:       "valid key prod mode",
			sessionKey: "this-is-a-32-character-long-key!",
			secure:     true,
			wantErr:    false,
		},
		{
			name:       "empty key",
			sessionKey: "",
			secure:     false,
			wantErr:    true,
		},
		{
			name:       "weak key dev mode",
			sessionKey: "short",
			secure:     false,
			wantErr:    false, // Warning but allowed in dev
		},
		{
			name:       "weak key prod mode",
			sessionKey: "short",
			secure:     true,
			wantErr:    true, // Error in prod
		},
		{
			name:       "default key prod mode",
			sessionKey: "dev-only-session-key-not-for-production",
			secure:     true,
			wantErr:    true, // Default keys not allowed in prod
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, err := NewSessionManager(tt.sessionKey, "test-session", "", time.Hour, tt.secure, logger)

			if tt.wantErr {
				if err == nil {
					t.Error("NewSessionManager() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("NewSessionManager() error = %v", err)
				}
				if sm == nil {
					t.Error("NewSessionManager() returned nil")
				}
			}
		})
	}
}

func TestSessionManager_SessionName(t *testing.T) {
	logger := zap.NewNop()

	// Default name
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)
	if sm.SessionName() != "stratasave-session" {
		t.Errorf("SessionName() = %q, want %q", sm.SessionName(), "stratasave-session")
	}

	// Custom name
	sm2, _ := NewSessionManager("this-is-a-32-character-long-key!", "custom-session", "", time.Hour, false, logger)
	if sm2.SessionName() != "custom-session" {
		t.Errorf("SessionName() = %q, want %q", sm2.SessionName(), "custom-session")
	}
}

func TestCurrentUser(t *testing.T) {
	// Request without user
	req := httptest.NewRequest("GET", "/", nil)
	user, ok := CurrentUser(req)
	if ok {
		t.Error("CurrentUser() should return false for request without user")
	}
	if user != nil {
		t.Error("CurrentUser() should return nil for request without user")
	}

	// Request with user
	testUser := &SessionUser{
		ID:      primitive.NewObjectID().Hex(),
		Name:    "Test User",
		LoginID: "test@example.com",
		Role:    "admin",
	}
	reqWithUser := WithTestUser(req, testUser)

	user, ok = CurrentUser(reqWithUser)
	if !ok {
		t.Error("CurrentUser() should return true for request with user")
	}
	if user == nil {
		t.Fatal("CurrentUser() should not return nil for request with user")
	}
	if user.ID != testUser.ID {
		t.Errorf("CurrentUser() ID = %q, want %q", user.ID, testUser.ID)
	}
	if user.Name != testUser.Name {
		t.Errorf("CurrentUser() Name = %q, want %q", user.Name, testUser.Name)
	}
}

func TestSessionUser_UserID(t *testing.T) {
	// Valid ID
	oid := primitive.NewObjectID()
	user := &SessionUser{ID: oid.Hex()}
	if user.UserID() != oid {
		t.Errorf("UserID() = %v, want %v", user.UserID(), oid)
	}

	// Invalid ID
	user2 := &SessionUser{ID: "invalid"}
	if !user2.UserID().IsZero() {
		t.Error("UserID() should return zero ObjectID for invalid ID")
	}

	// Empty ID
	user3 := &SessionUser{ID: ""}
	if !user3.UserID().IsZero() {
		t.Error("UserID() should return zero ObjectID for empty ID")
	}
}

func TestRequireSignedIn(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	// Handler that should only be called if authenticated
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	protected := sm.RequireSignedIn(handler)

	// Test without authentication - HTML request
	t.Run("unauthenticated HTML", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if called {
			t.Error("Handler should not be called for unauthenticated request")
		}
		if rec.Code != http.StatusSeeOther {
			t.Errorf("Status = %d, want %d (redirect)", rec.Code, http.StatusSeeOther)
		}
		location := rec.Header().Get("Location")
		if location == "" {
			t.Error("Should redirect to login")
		}
	})

	// Test without authentication - API request
	t.Run("unauthenticated API", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if called {
			t.Error("Handler should not be called for unauthenticated request")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	// Test with authentication
	t.Run("authenticated", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/protected", nil)
		req = WithTestUser(req, &SessionUser{
			ID:   primitive.NewObjectID().Hex(),
			Name: "Test",
			Role: "admin",
		})
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if !called {
			t.Error("Handler should be called for authenticated request")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestRequireRole(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Require admin role
	protected := sm.RequireRole("admin")(handler)

	// Test with correct role
	t.Run("correct role", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/admin", nil)
		req = WithTestUser(req, &SessionUser{
			ID:   primitive.NewObjectID().Hex(),
			Name: "Admin",
			Role: "admin",
		})
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if !called {
			t.Error("Handler should be called for user with correct role")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	// Test with wrong role - HTML request
	t.Run("wrong role HTML", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Accept", "text/html")
		req = WithTestUser(req, &SessionUser{
			ID:   primitive.NewObjectID().Hex(),
			Name: "User",
			Role: "user", // Wrong role
		})
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if called {
			t.Error("Handler should not be called for user with wrong role")
		}
		if rec.Code != http.StatusSeeOther {
			t.Errorf("Status = %d, want %d (redirect to forbidden)", rec.Code, http.StatusSeeOther)
		}
	})

	// Test with wrong role - API request
	t.Run("wrong role API", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Accept", "application/json")
		req = WithTestUser(req, &SessionUser{
			ID:   primitive.NewObjectID().Hex(),
			Name: "User",
			Role: "user",
		})
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if called {
			t.Error("Handler should not be called for user with wrong role")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	// Test unauthenticated
	t.Run("unauthenticated API", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if called {
			t.Error("Handler should not be called for unauthenticated request")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

func TestRequireRole_MultipleRoles(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Allow multiple roles
	protected := sm.RequireRole("admin", "editor")(handler)

	tests := []struct {
		name    string
		role    string
		allowed bool
	}{
		{"admin allowed", "admin", true},
		{"editor allowed", "editor", true},
		{"user denied", "user", false},
		{"viewer denied", "viewer", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Accept", "application/json")
			req = WithTestUser(req, &SessionUser{
				ID:   primitive.NewObjectID().Hex(),
				Name: "Test",
				Role: tt.role,
			})
			rec := httptest.NewRecorder()

			protected.ServeHTTP(rec, req)

			if tt.allowed && !called {
				t.Errorf("Handler should be called for role %q", tt.role)
			}
			if !tt.allowed && called {
				t.Errorf("Handler should not be called for role %q", tt.role)
			}
		})
	}
}

func TestIsDefaultKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"dev-only-key", true},
		{"change-me-please", true},
		{"placeholder-key", true},
		{"default-session-key", true},
		{"example-key-here", true},
		{"insecure-dev-key", true},
		{"test-key-123", true},
		{"secret123", true},
		{"password123", true},
		{"xK8nP2mQ9rT5vW7yB3cF6hJ0lN4sU1wZ", false}, // Random looking
		{"secure-random-key-that-is-long-enough", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := isDefaultKey(tt.key)
			if got != tt.want {
				t.Errorf("isDefaultKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestClassifySessionError(t *testing.T) {
	// Test nil error
	errType, _ := classifySessionError(nil)
	if errType != sessionErrUnknown {
		t.Errorf("classifySessionError(nil) type = %v, want %v", errType, sessionErrUnknown)
	}
}

func TestWantsHTML(t *testing.T) {
	tests := []struct {
		name      string
		accept    string
		hxRequest string
		want      bool
	}{
		{"HTML accept", "text/html", "", true},
		{"HTML with charset", "text/html; charset=utf-8", "", true},
		{"JSON accept", "application/json", "", false},
		{"HTMX request", "", "true", true},
		{"HTMX with JSON", "application/json", "true", true},
		{"Empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			if tt.hxRequest != "" {
				req.Header.Set("HX-Request", tt.hxRequest)
			}

			got := wantsHTML(req)
			if got != tt.want {
				t.Errorf("wantsHTML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionManager_Store(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	store := sm.Store()
	if store == nil {
		t.Error("Store() returned nil")
	}
}

func TestSessionManager_GetSession(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	req := httptest.NewRequest("GET", "/", nil)
	sess, err := sm.GetSession(req)
	if err != nil {
		t.Errorf("GetSession() error = %v", err)
	}
	if sess == nil {
		t.Error("GetSession() returned nil session")
	}
}

func TestSessionConfigError(t *testing.T) {
	err := &SessionConfigError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("SessionConfigError.Error() = %q, want %q", err.Error(), "test error")
	}
}

func TestSessionUser_SessionToken(t *testing.T) {
	user := &SessionUser{
		ID:    primitive.NewObjectID().Hex(),
		Token: "test-token-123",
	}
	if user.SessionToken() != "test-token-123" {
		t.Errorf("SessionToken() = %q, want %q", user.SessionToken(), "test-token-123")
	}
}

func TestRequireSignedIn_HTMX(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protected := sm.RequireSignedIn(handler)

	// HTMX request without authentication
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec.Header().Get("HX-Redirect") == "" {
		t.Error("Should set HX-Redirect header for HTMX request")
	}
}

func TestRequireRole_HTMX(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protected := sm.RequireRole("admin")(handler)

	// HTMX request without authentication
	t.Run("unauthenticated HTMX", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
		if rec.Header().Get("HX-Redirect") == "" {
			t.Error("Should set HX-Redirect header")
		}
	})

	// HTMX request with wrong role
	t.Run("wrong role HTMX", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("HX-Request", "true")
		req = WithTestUser(req, &SessionUser{
			ID:   primitive.NewObjectID().Hex(),
			Name: "User",
			Role: "user",
		})
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusForbidden)
		}
		if rec.Header().Get("HX-Redirect") == "" {
			t.Error("Should set HX-Redirect header for forbidden")
		}
	})
}

func TestRequireAuth_Alias(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// RequireAuth should behave the same as RequireSignedIn
	protected := sm.RequireAuth(handler)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("RequireAuth() Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestClassifySessionError_Types(t *testing.T) {
	// Test with various error message patterns
	tests := []struct {
		name     string
		errMsg   string
		wantType sessionErrorType
	}{
		{"expired", "expired timestamp", sessionErrExpired},
		{"mac invalid", "mac validation failed", sessionErrTampered},
		{"hash invalid", "hash mismatch", sessionErrTampered},
		{"decrypt failed", "decrypt error", sessionErrCorrupted},
		{"base64 error", "base64 decode failed", sessionErrCorrupted},
		{"decode error", "decode failed", sessionErrCorrupted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock securecookie decode error
			err := mockSecureCookieError{msg: tt.errMsg, isDecode: true}
			errType, _ := classifySessionError(err)
			if errType != tt.wantType {
				t.Errorf("classifySessionError() type = %v, want %v", errType, tt.wantType)
			}
		})
	}
}

func TestClassifySessionError_Backend(t *testing.T) {
	// Non-decode error should be backend
	err := mockSecureCookieError{msg: "backend error", isDecode: false}
	errType, category := classifySessionError(err)
	if errType != sessionErrBackend {
		t.Errorf("classifySessionError() type = %v, want %v", errType, sessionErrBackend)
	}
	if category != "backend" {
		t.Errorf("classifySessionError() category = %q, want %q", category, "backend")
	}
}

// mockSecureCookieError implements securecookie.Error for testing
type mockSecureCookieError struct {
	msg      string
	isDecode bool
}

func (e mockSecureCookieError) Error() string {
	return e.msg
}

func (e mockSecureCookieError) IsDecode() bool {
	return e.isDecode
}

func (e mockSecureCookieError) IsUsage() bool {
	return false
}

func (e mockSecureCookieError) IsInternal() bool {
	return false
}

func (e mockSecureCookieError) Cause() error {
	return nil
}

func TestCurrentURI(t *testing.T) {
	req := httptest.NewRequest("GET", "/test/path?query=value", nil)
	uri := currentURI(req)
	if uri != "/test/path?query=value" {
		t.Errorf("currentURI() = %q, want %q", uri, "/test/path?query=value")
	}
}

func TestGetString(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewSessionManager("this-is-a-32-character-long-key!", "", "", time.Hour, false, logger)

	req := httptest.NewRequest("GET", "/", nil)
	sess, _ := sm.GetSession(req)

	// Test with no value
	if got := getString(sess, "nonexistent"); got != "" {
		t.Errorf("getString() nonexistent = %q, want empty", got)
	}

	// Test with string value
	sess.Values["test_key"] = "test_value"
	if got := getString(sess, "test_key"); got != "test_value" {
		t.Errorf("getString() = %q, want %q", got, "test_value")
	}

	// Test with non-string value
	sess.Values["int_key"] = 123
	if got := getString(sess, "int_key"); got != "" {
		t.Errorf("getString() int = %q, want empty", got)
	}
}

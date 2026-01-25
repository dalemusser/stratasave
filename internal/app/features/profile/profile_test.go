package profile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/authutil"
	"github.com/dalemusser/stratasave/internal/testutil"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *mongo.Database, *userstore.Store, *sessions.Store) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	sessionsStore := sessions.New(db)
	errLog := errorsfeature.NewErrorLogger(logger)

	handler := NewHandler(db, sessionsStore, errLog, logger)

	return handler, db, userstore.New(db), sessionsStore
}

// createTestUser creates a user in the database and returns the user and their ID
func createTestUser(t *testing.T, users *userstore.Store, name, email, role, authMethod string) (primitive.ObjectID, string) {
	t.Helper()
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Hash a test password
	hash, err := authutil.HashPassword("TestPassword123!")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	input := userstore.CreateInput{
		FullName:     name,
		LoginID:      email,
		Role:         role,
		AuthMethod:   authMethod,
		PasswordHash: &hash,
	}

	user, err := users.CreateFromInput(ctx, input)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	return user.ID, email
}

func TestParseDevice(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{"empty", "", "Unknown Device"},
		{"iphone", "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)", "iPhone"},
		{"ipad", "Mozilla/5.0 (iPad; CPU OS 14_0 like Mac OS X)", "iPad"},
		{"android_phone", "Mozilla/5.0 (Linux; Android 10; Mobile)", "Android Phone"},
		{"android_tablet", "Mozilla/5.0 (Linux; Android 10; Tablet)", "Android Tablet"},
		{"windows_chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/90", "Windows (Chrome)"},
		{"windows_firefox", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Firefox/88", "Windows (Firefox)"},
		{"windows_edge", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Edge/90", "Windows (Edge)"},
		{"mac_safari", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15", "Mac (Safari)"},
		{"mac_chrome", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/90", "Mac (Chrome)"},
		{"mac_firefox", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:88.0) Firefox/88", "Mac (Firefox)"},
		{"linux_chrome", "Mozilla/5.0 (X11; Linux x86_64) Chrome/90", "Linux (Chrome)"},
		{"linux_firefox", "Mozilla/5.0 (X11; Linux x86_64; rv:88.0) Firefox/88", "Linux (Firefox)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDevice(tt.userAgent)
			if got != tt.want {
				t.Errorf("parseDevice(%q) = %q, want %q", tt.userAgent, got, tt.want)
			}
		})
	}
}

func TestFormatAuthMethod(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{"password", "Password"},
		{"email", "Email"},
		{"google", "Google"},
		{"trust", "Trusted"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := formatAuthMethod(tt.method)
			if got != tt.want {
				t.Errorf("formatAuthMethod(%q) = %q, want %q", tt.method, got, tt.want)
			}
		})
	}
}

func TestChangePassword_Success(t *testing.T) {
	h, _, users, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create test user with password auth
	userID, email := createTestUser(t, users, "Test User", "test@example.com", "admin", "password")

	// Create form data
	form := url.Values{
		"current_password": {"TestPassword123!"},
		"new_password":     {"NewPassword456!"},
		"confirm_password": {"NewPassword456!"},
	}

	req := httptest.NewRequest(http.MethodPost, "/profile/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
	})
	rec := httptest.NewRecorder()

	h.handleChangePassword(rec, req)

	// Should redirect with success message
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/profile?success=password" {
		t.Errorf("Location = %q, want %q", location, "/profile?success=password")
	}

	// Verify password was actually changed
	user, err := users.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if !authutil.CheckPassword("NewPassword456!", *user.PasswordHash) {
		t.Error("password was not changed")
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, users, _ := newTestHandler(t)

	// Create test user with password auth
	userID, email := createTestUser(t, users, "Test User", "test2@example.com", "admin", "password")

	// Create form data with wrong current password
	form := url.Values{
		"current_password": {"WrongPassword!"},
		"new_password":     {"NewPassword456!"},
		"confirm_password": {"NewPassword456!"},
	}

	req := httptest.NewRequest(http.MethodPost, "/profile/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
	})
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.handleChangePassword(rec, req)

	// Should NOT be a success redirect
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/profile?success=password" {
		t.Error("should not succeed with wrong current password")
	}
}

func TestChangePassword_Mismatch(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, users, _ := newTestHandler(t)

	// Create test user with password auth
	userID, email := createTestUser(t, users, "Test User", "test3@example.com", "admin", "password")

	// Create form data with mismatched passwords
	form := url.Values{
		"current_password": {"TestPassword123!"},
		"new_password":     {"NewPassword456!"},
		"confirm_password": {"DifferentPassword!"},
	}

	req := httptest.NewRequest(http.MethodPost, "/profile/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
	})
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.handleChangePassword(rec, req)

	// Should NOT succeed
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/profile?success=password" {
		t.Error("should not succeed with mismatched passwords")
	}
}

func TestChangePassword_TooWeak(t *testing.T) {
	h, _, users, _ := newTestHandler(t)

	// Create test user with password auth
	userID, email := createTestUser(t, users, "Test User", "test4@example.com", "admin", "password")

	// Create form data with weak new password
	form := url.Values{
		"current_password": {"TestPassword123!"},
		"new_password":     {"weak"},
		"confirm_password": {"weak"},
	}

	req := httptest.NewRequest(http.MethodPost, "/profile/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
	})
	rec := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected - template rendering
			}
		}()
		h.handleChangePassword(rec, req)
	}()

	// Should NOT succeed
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/profile?success=password" {
		t.Error("should not succeed with weak password")
	}
}

func TestChangePassword_SameAsCurrent(t *testing.T) {
	h, _, users, _ := newTestHandler(t)

	// Create test user with password auth
	userID, email := createTestUser(t, users, "Test User", "test5@example.com", "admin", "password")

	// Create form data with same password
	form := url.Values{
		"current_password": {"TestPassword123!"},
		"new_password":     {"TestPassword123!"},
		"confirm_password": {"TestPassword123!"},
	}

	req := httptest.NewRequest(http.MethodPost, "/profile/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
	})
	rec := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected - template rendering
			}
		}()
		h.handleChangePassword(rec, req)
	}()

	// Should NOT succeed
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/profile?success=password" {
		t.Error("should not succeed when reusing current password")
	}
}

func TestChangePassword_Unauthenticated(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	form := url.Values{
		"current_password": {"TestPassword123!"},
		"new_password":     {"NewPassword456!"},
		"confirm_password": {"NewPassword456!"},
	}

	req := httptest.NewRequest(http.MethodPost, "/profile/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No user in context
	rec := httptest.NewRecorder()

	h.handleChangePassword(rec, req)

	// Should redirect to login
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/login" {
		t.Errorf("Location = %q, want %q", location, "/login")
	}
}

func TestUpdatePreferences_Theme(t *testing.T) {
	h, _, users, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create test user
	userID, email := createTestUser(t, users, "Test User", "theme@example.com", "admin", "password")

	themes := []string{"light", "dark", "system"}

	for _, theme := range themes {
		t.Run(theme, func(t *testing.T) {
			form := url.Values{
				"theme_preference": {theme},
			}

			req := httptest.NewRequest(http.MethodPost, "/profile/preferences", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = auth.WithTestUser(req, &auth.SessionUser{
				ID:      userID.Hex(),
				Name:    "Test User",
				LoginID: email,
				Role:    "user",
			})
			rec := httptest.NewRecorder()

			h.handleUpdatePreferences(rec, req)

			// Should redirect with success
			if rec.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}

			location := rec.Header().Get("Location")
			if location != "/profile?success=preferences" {
				t.Errorf("Location = %q, want %q", location, "/profile?success=preferences")
			}

			// Verify theme was saved
			user, err := users.GetByID(ctx, userID)
			if err != nil {
				t.Fatalf("failed to get user: %v", err)
			}

			if user.ThemePreference != theme {
				t.Errorf("ThemePreference = %q, want %q", user.ThemePreference, theme)
			}
		})
	}
}

func TestUpdatePreferences_SetsCookie(t *testing.T) {
	h, _, users, _ := newTestHandler(t)

	// Create test user
	userID, email := createTestUser(t, users, "Test User", "cookie@example.com", "admin", "password")

	form := url.Values{
		"theme_preference": {"dark"},
	}

	req := httptest.NewRequest(http.MethodPost, "/profile/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
	})
	rec := httptest.NewRecorder()

	h.handleUpdatePreferences(rec, req)

	// Check for theme_pref cookie
	cookies := rec.Result().Cookies()
	var themeCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "theme_pref" {
			themeCookie = c
			break
		}
	}

	if themeCookie == nil {
		t.Error("theme_pref cookie not set")
	} else if themeCookie.Value != "dark" {
		t.Errorf("theme_pref cookie = %q, want %q", themeCookie.Value, "dark")
	}
}

func TestUpdatePreferences_InvalidTheme(t *testing.T) {
	h, _, users, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create test user
	userID, email := createTestUser(t, users, "Test User", "invalid@example.com", "admin", "password")

	form := url.Values{
		"theme_preference": {"invalid_theme"},
	}

	req := httptest.NewRequest(http.MethodPost, "/profile/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
	})
	rec := httptest.NewRecorder()

	h.handleUpdatePreferences(rec, req)

	// Should still succeed (defaults to "system")
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// Verify it defaulted to "system"
	user, err := users.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if user.ThemePreference != "system" {
		t.Errorf("ThemePreference = %q, want %q (should default)", user.ThemePreference, "system")
	}
}

func TestRevokeSession_Success(t *testing.T) {
	h, _, users, sessionsStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create test user
	userID, email := createTestUser(t, users, "Test User", "revoke@example.com", "admin", "password")

	// Create two sessions - one to keep, one to revoke
	currentToken := "current-session-token"
	sessionToRevoke := sessions.Session{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Token:     "session-to-revoke",
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := sessionsStore.Create(ctx, sessionToRevoke)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create request with chi URL param
	req := httptest.NewRequest(http.MethodPost, "/profile/sessions/"+sessionToRevoke.ID.Hex()+"/revoke", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionToRevoke.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
		Token:   currentToken, // Different from session being revoked
	})
	rec := httptest.NewRecorder()

	h.revokeSession(rec, req)

	// Should redirect with success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/profile?success=revoked" {
		t.Errorf("Location = %q, want %q", location, "/profile?success=revoked")
	}

	// Verify session was deleted
	_, err = sessionsStore.GetByID(ctx, sessionToRevoke.ID)
	if err == nil {
		t.Error("session should have been deleted")
	}
}

func TestRevokeSession_CurrentSession(t *testing.T) {
	h, _, users, sessionsStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create test user
	userID, email := createTestUser(t, users, "Test User", "revoke2@example.com", "admin", "password")

	// Create session
	currentToken := "my-current-token"
	session := sessions.Session{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Token:     currentToken,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := sessionsStore.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Try to revoke the current session
	req := httptest.NewRequest(http.MethodPost, "/profile/sessions/"+session.ID.Hex()+"/revoke", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", session.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
		Token:   currentToken, // Same as session being revoked
	})
	rec := httptest.NewRecorder()

	h.revokeSession(rec, req)

	// Should redirect with error
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/profile?error=use_logout" {
		t.Errorf("Location = %q, want %q", location, "/profile?error=use_logout")
	}
}

func TestRevokeSession_NotOwned(t *testing.T) {
	h, _, users, sessionsStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create two users
	user1ID, user1Email := createTestUser(t, users, "User 1", "user1@example.com", "admin", "password")
	user2ID, _ := createTestUser(t, users, "User 2", "user2@example.com", "admin", "password")

	// Create session for user2
	session := sessions.Session{
		ID:        primitive.NewObjectID(),
		UserID:    user2ID,
		Token:     "user2-session",
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := sessionsStore.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// User1 tries to revoke user2's session
	req := httptest.NewRequest(http.MethodPost, "/profile/sessions/"+session.ID.Hex()+"/revoke", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", session.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      user1ID.Hex(),
		Name:    "User 1",
		LoginID: user1Email,
		Role:    "user",
		Token:   "user1-session",
	})
	rec := httptest.NewRecorder()

	h.revokeSession(rec, req)

	// Should return forbidden
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRevokeAllSessions_Success(t *testing.T) {
	h, _, users, sessionsStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create test user
	userID, email := createTestUser(t, users, "Test User", "revokeall@example.com", "admin", "password")

	// Create multiple sessions
	currentToken := "current-session-token"
	sessions1 := sessions.Session{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Token:     "session-1",
		IPAddress: "127.0.0.1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	sessions2 := sessions.Session{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Token:     "session-2",
		IPAddress: "127.0.0.2",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	currentSession := sessions.Session{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Token:     currentToken,
		IPAddress: "127.0.0.3",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	for _, s := range []sessions.Session{sessions1, sessions2, currentSession} {
		if err := sessionsStore.Create(ctx, s); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
	}

	// Create session manager for the handler
	logger := zap.NewNop()
	sessionMgr, err := auth.NewSessionManager(
		"test-session-key-for-testing-1234567890",
		"test-session",
		"",
		24*time.Hour,
		false,
		logger,
	)
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/profile/sessions/revoke-all", nil)
	req = auth.WithTestUser(req, &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: email,
		Role:    "user",
		Token:   currentToken,
	})
	rec := httptest.NewRecorder()

	// Call the handler
	handlerFunc := h.revokeAllSessions(sessionMgr)
	handlerFunc(rec, req)

	// Should redirect with success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/profile?success=revoked_all" {
		t.Errorf("Location = %q, want %q", location, "/profile?success=revoked_all")
	}

	// Verify other sessions were deleted but current session remains
	remainingSessions, err := sessionsStore.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	if len(remainingSessions) != 1 {
		t.Errorf("expected 1 remaining session, got %d", len(remainingSessions))
	}

	if len(remainingSessions) > 0 && remainingSessions[0].Token != currentToken {
		t.Error("wrong session remained")
	}
}

func TestRoutes(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	logger := zap.NewNop()

	sessionMgr, err := auth.NewSessionManager(
		"test-session-key-for-testing-1234567890",
		"test-session",
		"",
		24*time.Hour,
		false,
		logger,
	)
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	router := Routes(h, sessionMgr)
	if router == nil {
		t.Fatal("Routes() returned nil")
	}
}

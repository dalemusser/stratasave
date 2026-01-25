package logout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *sessions.Store, *auth.SessionManager) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	sessionsStore := sessions.New(db)

	// Create session manager for tests
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

	// auditLogger can be nil - it's nil-safe
	handler := NewHandler(sessionMgr, nil, sessionsStore, logger)

	return handler, sessionsStore, sessionMgr
}

func TestLogout_RedirectsToRoot(t *testing.T) {
	h, _, _ := newTestHandler(t)

	// Create authenticated request
	user := testutil.AdminUser()
	req := testutil.NewAuthenticatedRequest(http.MethodPost, "/logout", user)
	rec := httptest.NewRecorder()

	h.handleLogout(rec, req)

	// Verify redirect to root
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/" {
		t.Errorf("Location = %q, want %q", location, "/")
	}
}

func TestLogout_GET(t *testing.T) {
	h, _, _ := newTestHandler(t)

	// GET requests should also work (for simple logout links)
	user := testutil.AdminUser()
	req := testutil.NewAuthenticatedRequest(http.MethodGet, "/logout", user)
	rec := httptest.NewRecorder()

	h.handleLogout(rec, req)

	// Verify redirect
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/" {
		t.Errorf("Location = %q, want %q", location, "/")
	}
}

func TestLogout_ClosesSessionInDB(t *testing.T) {
	h, sessionsStore, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user and session in the database
	userID := primitive.NewObjectID()
	token := "test-session-token-12345"

	// Create session in store
	session := sessions.Session{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Token:     token,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err := sessionsStore.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Verify session is active (GetByToken only returns active sessions)
	found, err := sessionsStore.GetByToken(ctx, token)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if found.LogoutAt != nil {
		t.Error("session should be active before logout (LogoutAt should be nil)")
	}

	// Create authenticated request with the session token
	sessionUser := &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: "test@example.com",
		Role:    "admin",
		Token:   token,
	}
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.handleLogout(rec, req)

	// Verify session is closed in DB
	closed, err := sessionsStore.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to get session after logout: %v", err)
	}

	if closed.LogoutAt == nil {
		t.Error("LogoutAt should be set after logout")
	}

	if closed.EndReason != sessions.EndReasonLogout {
		t.Errorf("end_reason = %q, want %q", closed.EndReason, sessions.EndReasonLogout)
	}
}

func TestLogout_NoUserInContext(t *testing.T) {
	h, _, _ := newTestHandler(t)

	// Request without user in context
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	h.handleLogout(rec, req)

	// Should still redirect (graceful handling)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/" {
		t.Errorf("Location = %q, want %q", location, "/")
	}
}

func TestRoutes(t *testing.T) {
	h, _, sessionMgr := newTestHandler(t)

	router := Routes(h, sessionMgr)
	if router == nil {
		t.Fatal("Routes() returned nil")
	}
}

func TestLogout_WithSessionTokenMissing(t *testing.T) {
	h, _, _ := newTestHandler(t)
	ctx := context.Background()
	_ = ctx // suppress unused warning

	// User without session token
	sessionUser := &auth.SessionUser{
		ID:      primitive.NewObjectID().Hex(),
		Name:    "Test User",
		LoginID: "test@example.com",
		Role:    "admin",
		Token:   "", // Empty token
	}
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	// Should not panic, should still redirect
	h.handleLogout(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

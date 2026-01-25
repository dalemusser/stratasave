package heartbeat

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/activity"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *sessions.Store) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	sessionsStore := sessions.New(db)
	activityStore := activity.New(db)

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

	handler := NewHandler(sessionsStore, activityStore, sessionMgr, logger)

	return handler, sessionsStore
}

func TestNewHandler(t *testing.T) {
	h, _ := newTestHandler(t)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestSetIdleLogoutConfig(t *testing.T) {
	h, _ := newTestHandler(t)

	h.SetIdleLogoutConfig(true, 30*time.Minute, 5*time.Minute)

	if !h.IdleLogoutEnabled {
		t.Error("IdleLogoutEnabled should be true")
	}
	if h.IdleLogoutTimeout != 30*time.Minute {
		t.Errorf("IdleLogoutTimeout = %v, want %v", h.IdleLogoutTimeout, 30*time.Minute)
	}
	if h.IdleLogoutWarning != 5*time.Minute {
		t.Errorf("IdleLogoutWarning = %v, want %v", h.IdleLogoutWarning, 5*time.Minute)
	}
}

func TestHeartbeat_Unauthenticated(t *testing.T) {
	h, _ := newTestHandler(t)

	// Request without user in context
	req := httptest.NewRequest(http.MethodPost, "/heartbeat", nil)
	rec := httptest.NewRecorder()

	h.ServeHeartbeat(rec, req)

	// Should return OK (silent fail)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHeartbeat_NoSessionToken(t *testing.T) {
	h, _ := newTestHandler(t)

	// User without session token
	sessionUser := &auth.SessionUser{
		ID:      primitive.NewObjectID().Hex(),
		Name:    "Test User",
		LoginID: "test@example.com",
		Role:    "admin",
		Token:   "", // Empty token
	}

	req := httptest.NewRequest(http.MethodPost, "/heartbeat", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.ServeHeartbeat(rec, req)

	// Should return OK (silent fail)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHeartbeat_InvalidSession(t *testing.T) {
	h, _ := newTestHandler(t)

	// User with non-existent session token
	sessionUser := &auth.SessionUser{
		ID:      primitive.NewObjectID().Hex(),
		Name:    "Test User",
		LoginID: "test@example.com",
		Role:    "admin",
		Token:   "non-existent-token",
	}

	req := httptest.NewRequest(http.MethodPost, "/heartbeat", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.ServeHeartbeat(rec, req)

	// Should return 401 (session not found)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHeartbeat_ValidSession(t *testing.T) {
	h, sessionsStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	token := "valid-session-token"

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

	sessionUser := &auth.SessionUser{
		ID:      userID.Hex(),
		Name:    "Test User",
		LoginID: "test@example.com",
		Role:    "admin",
		Token:   token,
	}

	req := httptest.NewRequest(http.MethodPost, "/heartbeat", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.ServeHeartbeat(rec, req)

	// Should return OK
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRoutes(t *testing.T) {
	h, _ := newTestHandler(t)
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

func TestClientIP(t *testing.T) {
	tests := []struct {
		name           string
		xForwardedFor  string
		xRealIP        string
		remoteAddr     string
		expectedResult string
	}{
		{
			name:           "X-Forwarded-For with single IP",
			xForwardedFor:  "192.168.1.1",
			expectedResult: "192.168.1.1",
		},
		{
			name:           "X-Forwarded-For with multiple IPs",
			xForwardedFor:  "192.168.1.1, 10.0.0.1",
			expectedResult: "192.168.1.1",
		},
		{
			name:           "X-Real-IP only",
			xRealIP:        "192.168.1.2",
			expectedResult: "192.168.1.2",
		},
		{
			name:           "RemoteAddr only",
			remoteAddr:     "192.168.1.3:12345",
			expectedResult: "192.168.1.3:12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}

			got := clientIP(req)
			if got != tt.expectedResult {
				t.Errorf("clientIP() = %q, want %q", got, tt.expectedResult)
			}
		})
	}
}

package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

func TestNewHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, logger)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, logger)

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

func TestDashboard_Unauthenticated(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, logger)

	// Request without user in context
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	h.showDashboard(rec, req)

	// Should redirect to login
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/login" {
		t.Errorf("Location = %q, want %q", location, "/login")
	}
}

func TestDashboard_AdminView(t *testing.T) {
	testutil.MustBootTemplates(t)
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, logger)

	// Admin user
	sessionUser := &auth.SessionUser{
		ID:      primitive.NewObjectID().Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.showDashboard(rec, req)

	// Admin user should get 200 OK (not redirected to login)
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/login" {
		t.Error("admin user should not be redirected to login")
	}
}

func TestDashboard_UserView(t *testing.T) {
	testutil.MustBootTemplates(t)
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, logger)

	// Regular user (using admin role since that's the only role in strata)
	sessionUser := &auth.SessionUser{
		ID:      primitive.NewObjectID().Hex(),
		Name:    "Regular User",
		LoginID: "user@example.com",
		Role:    "user", // Non-admin role
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.showDashboard(rec, req)

	// User should not be redirected to login
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/login" {
		t.Error("user should not be redirected to login")
	}
}

func TestDashboardVM(t *testing.T) {
	// Test that DashboardVM struct works as expected
	vm := DashboardVM{}
	vm.Title = "Test Dashboard"

	if vm.Title != "Test Dashboard" {
		t.Errorf("Title = %q, want %q", vm.Title, "Test Dashboard")
	}
}

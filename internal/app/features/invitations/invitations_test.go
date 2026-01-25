package invitations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/invitation"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *mongo.Database, *invitation.Store, *userstore.Store) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	sessionsStore := sessions.New(db)
	invitationStore := invitation.New(db, 7*24*time.Hour)

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

	handler := NewHandler(
		db,
		sessionMgr,
		sessionsStore,
		nil, // errLog
		nil, // mailer
		nil, // auditLogger
		"http://localhost:8080",
		7*24*time.Hour,
		logger,
	)

	return handler, db, invitationStore, userstore.New(db)
}

func TestNewHandler(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestAdminRoutes(t *testing.T) {
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

	router := AdminRoutes(h, sessionMgr)
	if router == nil {
		t.Fatal("AdminRoutes() returned nil")
	}
}

func TestAcceptRoutes(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	router := AcceptRoutes(h)
	if router == nil {
		t.Fatal("AcceptRoutes() returned nil")
	}
}

func TestList_AdminOnly(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _, _ := newTestHandler(t)

	// Request without user (unauthenticated)
	req := httptest.NewRequest(http.MethodGet, "/invitations", nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.list(rec, req)

	// Handler should handle unauthenticated request gracefully
}

func TestList_ReturnsInvitations(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, db, invStore, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	adminID := primitive.NewObjectID()

	// Create a test invitation
	_, err := invStore.Create(ctx, invitation.CreateInput{
		Email:     "invitee@example.com",
		Role:      "admin",
		InvitedBy: adminID,
	})
	if err != nil {
		t.Fatalf("failed to create test invitation: %v", err)
	}

	// Create admin user for context
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/invitations", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	_ = db
}

func TestCreate_InvalidEmail(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	form := url.Values{}
	form.Set("email", "not-a-valid-email")
	form.Set("role", "admin")

	req := httptest.NewRequest(http.MethodPost, "/invitations/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	// Invalid email should not redirect (should show error in form)
	if rec.Code == http.StatusSeeOther {
		t.Error("invalid email should not result in redirect")
	}
}

func TestCreate_DuplicateEmail(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create an existing user
	existingUser, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "Existing User",
		Email:      "existing@example.com",
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create existing user: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	form := url.Values{}
	form.Set("email", *existingUser.Email)
	form.Set("role", "admin")

	req := httptest.NewRequest(http.MethodPost, "/invitations/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	// Duplicate email should not redirect to success
	if rec.Code == http.StatusSeeOther && strings.Contains(rec.Header().Get("Location"), "success=1") {
		t.Error("duplicate email should not result in success redirect")
	}
}

func TestCreate_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	form := url.Values{}
	form.Set("email", "newinvitee@example.com")
	form.Set("role", "admin")

	req := httptest.NewRequest(http.MethodPost, "/invitations/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	// Should redirect to list with success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "success=1") {
		t.Errorf("Location = %q, want to contain 'success=1'", location)
	}
}

func TestRevoke_Success(t *testing.T) {
	h, _, invStore, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	adminID := primitive.NewObjectID()

	// Create a test invitation
	inv, err := invStore.Create(ctx, invitation.CreateInput{
		Email:     "torevoke@example.com",
		Role:      "admin",
		InvitedBy: adminID,
	})
	if err != nil {
		t.Fatalf("failed to create test invitation: %v", err)
	}

	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/invitations/"+inv.ID.Hex()+"/revoke", nil)
	req = auth.WithTestUser(req, sessionUser)

	// Add chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", inv.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.revoke(rec, req)

	// Should redirect with revoked=1
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "revoked=1") {
		t.Errorf("Location = %q, want to contain 'revoked=1'", location)
	}

	// Verify invitation is revoked in DB
	revokedInv, err := invStore.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed to get revoked invitation: %v", err)
	}
	if !revokedInv.Revoked {
		t.Error("invitation should be marked as revoked")
	}
}

func TestRevoke_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	nonExistentID := primitive.NewObjectID()

	req := httptest.NewRequest(http.MethodPost, "/invitations/"+nonExistentID.Hex()+"/revoke", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", nonExistentID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.revoke(rec, req)

	// Should return 404
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestResend_Success(t *testing.T) {
	h, _, invStore, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	adminID := primitive.NewObjectID()

	// Create a test invitation
	inv, err := invStore.Create(ctx, invitation.CreateInput{
		Email:     "toresend@example.com",
		Role:      "admin",
		InvitedBy: adminID,
	})
	if err != nil {
		t.Fatalf("failed to create test invitation: %v", err)
	}

	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/invitations/"+inv.ID.Hex()+"/resend", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", inv.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.resend(rec, req)

	// Should redirect with resent=1
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "resent=1") {
		t.Errorf("Location = %q, want to contain 'resent=1'", location)
	}
}

func TestShowAccept_InvalidToken(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/invite?token=invalid-token", nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.showAccept(rec, req)

	// Should show error page (not redirect)
	if rec.Code == http.StatusSeeOther {
		t.Error("invalid token should not redirect")
	}
}

func TestShowAccept_ValidToken(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, invStore, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	adminID := primitive.NewObjectID()

	// Create a test invitation
	inv, err := invStore.Create(ctx, invitation.CreateInput{
		Email:     "validtoken@example.com",
		Role:      "admin",
		InvitedBy: adminID,
	})
	if err != nil {
		t.Fatalf("failed to create test invitation: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/invite?token="+inv.Token, nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.showAccept(rec, req)

	// Should not redirect to login (should show accept form)
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/login" {
		t.Error("valid token should not redirect to login")
	}
}

func TestHandleAccept_MissingName(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, invStore, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	adminID := primitive.NewObjectID()

	// Create a test invitation
	inv, err := invStore.Create(ctx, invitation.CreateInput{
		Email:     "missingname@example.com",
		Role:      "admin",
		InvitedBy: adminID,
	})
	if err != nil {
		t.Fatalf("failed to create test invitation: %v", err)
	}

	form := url.Values{}
	form.Set("token", inv.Token)
	form.Set("full_name", "") // Missing name

	req := httptest.NewRequest(http.MethodPost, "/invite", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.handleAccept(rec, req)

	// Should not redirect to dashboard (should show error)
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/dashboard" {
		t.Error("missing name should not redirect to dashboard")
	}
}

func TestHandleAccept_InvalidToken(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _, _ := newTestHandler(t)

	form := url.Values{}
	form.Set("token", "invalid-token")
	form.Set("full_name", "Test User")

	req := httptest.NewRequest(http.MethodPost, "/invite", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.handleAccept(rec, req)

	// Should not redirect to dashboard
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/dashboard" {
		t.Error("invalid token should not redirect to dashboard")
	}
}

func TestHandleAccept_CreatesUser(t *testing.T) {
	h, _, invStore, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	adminID := primitive.NewObjectID()

	// Create a test invitation
	inv, err := invStore.Create(ctx, invitation.CreateInput{
		Email:     "newuser@example.com",
		Role:      "admin",
		InvitedBy: adminID,
	})
	if err != nil {
		t.Fatalf("failed to create test invitation: %v", err)
	}

	form := url.Values{}
	form.Set("token", inv.Token)
	form.Set("full_name", "New User")

	req := httptest.NewRequest(http.MethodPost, "/invite", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.handleAccept(rec, req)

	// Should redirect to dashboard on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/dashboard" && !strings.Contains(location, "login") {
		t.Errorf("Location = %q, want '/dashboard' or login redirect", location)
	}

	// Verify user was created
	user, err := userStore.GetByEmail(ctx, "newuser@example.com")
	if err != nil {
		t.Fatalf("failed to get created user: %v", err)
	}
	if user == nil {
		t.Error("user should have been created")
	}
	if user != nil && user.FullName != "New User" {
		t.Errorf("user.FullName = %q, want %q", user.FullName, "New User")
	}

	// Verify invitation was marked as used
	usedInv, err := invStore.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed to get invitation: %v", err)
	}
	if usedInv.UsedAt == nil {
		t.Error("invitation should be marked as used")
	}
}

func TestManageModal_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	nonExistentID := primitive.NewObjectID()

	req := httptest.NewRequest(http.MethodGet, "/invitations/"+nonExistentID.Hex()+"/manage_modal", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", nonExistentID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.manageModal(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestManageModal_InvalidID(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/invitations/invalid-id/manage_modal", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.manageModal(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListVM(t *testing.T) {
	// Test that ListVM struct works as expected
	vm := ListVM{}
	vm.Title = "Test Invitations"
	vm.Success = "Test success"
	vm.Error = "Test error"
	vm.Invitations = []invitationRow{
		{ID: "test-id", Email: "test@example.com", Role: "admin"},
	}

	if vm.Title != "Test Invitations" {
		t.Errorf("Title = %q, want %q", vm.Title, "Test Invitations")
	}
	if vm.Success != "Test success" {
		t.Errorf("Success = %q, want %q", vm.Success, "Test success")
	}
	if vm.Error != "Test error" {
		t.Errorf("Error = %q, want %q", vm.Error, "Test error")
	}
	if len(vm.Invitations) != 1 {
		t.Errorf("len(Invitations) = %d, want 1", len(vm.Invitations))
	}
}

func TestAcceptVM(t *testing.T) {
	// Test that AcceptVM struct works as expected
	vm := AcceptVM{}
	vm.Title = "Accept Invitation"
	vm.Token = "test-token"
	vm.Email = "test@example.com"
	vm.FullName = "Test User"
	vm.Error = "Test error"

	if vm.Token != "test-token" {
		t.Errorf("Token = %q, want %q", vm.Token, "test-token")
	}
	if vm.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", vm.Email, "test@example.com")
	}
	if vm.FullName != "Test User" {
		t.Errorf("FullName = %q, want %q", vm.FullName, "Test User")
	}
}

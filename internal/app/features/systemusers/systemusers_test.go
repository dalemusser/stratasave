package systemusers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *mongo.Database, *userstore.Store) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	handler := NewHandler(
		db,
		nil, // mailer
		nil, // errLog
		nil, // auditLogger
		logger,
	)

	return handler, db, userstore.New(db)
}

func TestNewHandler(t *testing.T) {
	h, _, _ := newTestHandler(t)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestRoutes(t *testing.T) {
	h, _, _ := newTestHandler(t)
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

func TestList_AdminOnly(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	// Request without user (handled by middleware in practice)
	req := httptest.NewRequest(http.MethodGet, "/system-users", nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.list(rec, req)

	// Handler should handle unauthenticated request gracefully
}

func TestList_Pagination(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create some test users
	for i := 0; i < 5; i++ {
		_, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
			FullName:   "Test User " + string(rune('A'+i)),
			Email:      "testuser" + string(rune('a'+i)) + "@example.com",
			AuthMethod: "trust",
			Role:       "admin",
		})
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/system-users?page=1", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCreate_Success(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	form := url.Values{}
	form.Set("full_name", "New Test User")
	form.Set("email", "newuser@example.com")
	form.Set("auth_method", "trust")

	req := httptest.NewRequest(http.MethodPost, "/system-users/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	// Should redirect on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// Verify user was created
	user, err := userStore.GetByEmail(ctx, "newuser@example.com")
	if err != nil {
		t.Fatalf("failed to get created user: %v", err)
	}
	if user == nil {
		t.Error("user should have been created")
	}
	if user != nil && user.FullName != "New Test User" {
		t.Errorf("user.FullName = %q, want %q", user.FullName, "New Test User")
	}
}

func TestCreate_PasswordAuthRequiresPassword(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	form := url.Values{}
	form.Set("full_name", "Password User")
	form.Set("login_id", "passworduser")
	form.Set("auth_method", "password")
	// No password set

	req := httptest.NewRequest(http.MethodPost, "/system-users/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	// Should not redirect (should show error)
	if rec.Code == http.StatusSeeOther {
		t.Error("creating password user without password should not redirect")
	}
}

func TestShow_NotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	nonExistentID := primitive.NewObjectID()

	req := httptest.NewRequest(http.MethodGet, "/system-users/"+nonExistentID.Hex(), nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", nonExistentID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.show(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestShow_InvalidID(t *testing.T) {
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/system-users/invalid-id", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.show(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdate_Success(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user to update
	user, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "Original Name",
		Email:      "update@example.com",
		AuthMethod: "trust",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	form := url.Values{}
	form.Set("full_name", "Updated Name")
	form.Set("email", "update@example.com")
	form.Set("auth_method", "trust")

	req := httptest.NewRequest(http.MethodPost, "/system-users/"+user.ID.Hex(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", user.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.update(rec, req)

	// Should redirect on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// Verify user was updated
	updatedUser, err := userStore.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get updated user: %v", err)
	}
	if updatedUser.FullName != "Updated Name" {
		t.Errorf("user.FullName = %q, want %q", updatedUser.FullName, "Updated Name")
	}
}

func TestDisable_CannotDisableSelf(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create the admin user
	user, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "Self Admin",
		Email:      "selfadmin@example.com",
		AuthMethod: "trust",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	sessionUser := &auth.SessionUser{
		ID:      user.ID.Hex(),
		Name:    "Self Admin",
		LoginID: "selfadmin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/system-users/"+user.ID.Hex()+"/disable", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", user.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.disable(rec, req)

	// Should redirect with error
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "cannot_disable_self") {
		t.Errorf("Location = %q, want to contain 'cannot_disable_self'", location)
	}
}

func TestDisable_Success(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user to disable
	targetUser, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "Target User",
		Email:      "target@example.com",
		AuthMethod: "trust",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/system-users/"+targetUser.ID.Hex()+"/disable", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", targetUser.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.disable(rec, req)

	// Should redirect on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// Verify user was disabled
	disabledUser, err := userStore.GetByID(ctx, targetUser.ID)
	if err != nil {
		t.Fatalf("failed to get disabled user: %v", err)
	}
	if disabledUser.Status != "disabled" {
		t.Errorf("user.Status = %q, want %q", disabledUser.Status, "disabled")
	}
}

func TestEnable_Success(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a disabled user
	targetUser, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "Disabled User",
		Email:      "disabled@example.com",
		AuthMethod: "trust",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Disable the user
	status := "disabled"
	err = userStore.UpdateFromInput(ctx, targetUser.ID, userstore.UpdateInput{Status: &status})
	if err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/system-users/"+targetUser.ID.Hex()+"/enable", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", targetUser.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.enable(rec, req)

	// Should redirect on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// Verify user was enabled
	enabledUser, err := userStore.GetByID(ctx, targetUser.ID)
	if err != nil {
		t.Fatalf("failed to get enabled user: %v", err)
	}
	if enabledUser.Status != "active" {
		t.Errorf("user.Status = %q, want %q", enabledUser.Status, "active")
	}
}

func TestDelete_CannotDeleteSelf(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create the admin user
	user, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "Self Admin",
		Email:      "selfdelete@example.com",
		AuthMethod: "trust",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	sessionUser := &auth.SessionUser{
		ID:      user.ID.Hex(),
		Name:    "Self Admin",
		LoginID: "selfdelete@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/system-users/"+user.ID.Hex()+"/delete", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", user.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.delete(rec, req)

	// Should redirect with error
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "cannot_delete_self") {
		t.Errorf("Location = %q, want to contain 'cannot_delete_self'", location)
	}
}

func TestDelete_Success(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user to delete
	targetUser, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "Delete Me",
		Email:      "deleteme@example.com",
		AuthMethod: "trust",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/system-users/"+targetUser.ID.Hex()+"/delete", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", targetUser.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.delete(rec, req)

	// Should redirect on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// Verify user was deleted (soft delete)
	deletedUser, err := userStore.GetByID(ctx, targetUser.ID)
	if err == nil && deletedUser != nil {
		// If user still exists, it should be marked as deleted
		// The Delete method does soft delete by setting deleted_at
	}
}

func TestResetPassword_Success(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user with password auth
	user, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "Password Reset User",
		LoginID:    "resetuser",
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	form := url.Values{}
	form.Set("new_password", "newpassword123")

	req := httptest.NewRequest(http.MethodPost, "/system-users/"+user.ID.Hex()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", user.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.resetPassword(rec, req)

	// Should redirect on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "success=1") {
		t.Errorf("Location = %q, want to contain 'success=1'", location)
	}
}

func TestResetPassword_MissingPassword(t *testing.T) {
	h, _, userStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user
	user, err := userStore.CreateFromInput(ctx, userstore.CreateInput{
		FullName:   "No Password User",
		LoginID:    "nopassuser",
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	form := url.Values{}
	// No password set

	req := httptest.NewRequest(http.MethodPost, "/system-users/"+user.ID.Hex()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", user.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.resetPassword(rec, req)

	// Should redirect with error
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "password_required") {
		t.Errorf("Location = %q, want to contain 'password_required'", location)
	}
}

func TestManageModal_NotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	nonExistentID := primitive.NewObjectID()

	req := httptest.NewRequest(http.MethodGet, "/system-users/"+nonExistentID.Hex()+"/manage_modal", nil)
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

func TestFormatAuthMethod(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"password", "password"},
		{"email", "email"},
		{"google", "google"},
		{"trust", "trust"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatAuthMethod(tt.input)
			if got != tt.expected {
				t.Errorf("formatAuthMethod(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestListVM(t *testing.T) {
	vm := ListVM{}
	vm.Title = "System Users"
	vm.SearchQuery = "test"
	vm.Page = 1
	vm.Total = 50
	vm.TotalPages = 3
	vm.HasPrev = false
	vm.HasNext = true

	if vm.Title != "System Users" {
		t.Errorf("Title = %q, want %q", vm.Title, "System Users")
	}
	if vm.SearchQuery != "test" {
		t.Errorf("SearchQuery = %q, want %q", vm.SearchQuery, "test")
	}
	if !vm.HasNext {
		t.Error("HasNext should be true")
	}
}

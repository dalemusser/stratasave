package announcements

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/announcement"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *mongo.Database, *announcement.Store) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	handler := NewHandler(db, nil, logger)

	return handler, db, announcement.New(db)
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

func TestViewRoutes(t *testing.T) {
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

	router := ViewRoutes(h, sessionMgr)
	if router == nil {
		t.Fatal("ViewRoutes() returned nil")
	}
}

func TestList_AdminOnly(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/announcements", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetActiveAnnouncements_FiltersInactive(t *testing.T) {
	h, _, annStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create an active announcement
	_, err := annStore.Create(ctx, announcement.CreateInput{
		Title:   "Active Announcement",
		Content: "This is active",
		Type:    announcement.TypeInfo,
		Active:  true,
	})
	if err != nil {
		t.Fatalf("failed to create active announcement: %v", err)
	}

	// Create an inactive announcement
	_, err = annStore.Create(ctx, announcement.CreateInput{
		Title:   "Inactive Announcement",
		Content: "This is inactive",
		Type:    announcement.TypeWarning,
		Active:  false,
	})
	if err != nil {
		t.Fatalf("failed to create inactive announcement: %v", err)
	}

	// Get active announcements
	active, err := h.GetActiveAnnouncements(ctx)
	if err != nil {
		t.Fatalf("GetActiveAnnouncements() error: %v", err)
	}

	// Should only return active announcements
	for _, ann := range active {
		if !ann.Active {
			t.Error("GetActiveAnnouncements() returned inactive announcement")
		}
	}
}

func TestGetActiveAnnouncements_Empty(t *testing.T) {
	h, _, _ := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Get active announcements (none created)
	active, err := h.GetActiveAnnouncements(ctx)
	if err != nil {
		t.Fatalf("GetActiveAnnouncements() error: %v", err)
	}

	// Should return empty slice
	if len(active) != 0 {
		t.Errorf("GetActiveAnnouncements() returned %d, want 0", len(active))
	}
}

func TestGetStore(t *testing.T) {
	h, _, _ := newTestHandler(t)
	store := h.GetStore()
	if store == nil {
		t.Fatal("GetStore() returned nil")
	}
}

func TestCreate_Success(t *testing.T) {
	h, _, annStore := newTestHandler(t)
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
	form.Set("title", "New Announcement")
	form.Set("content", "Test content")
	form.Set("type", "info")
	form.Set("dismissible", "on")
	form.Set("active", "on")

	req := httptest.NewRequest(http.MethodPost, "/announcements/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	// Should redirect on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "success=created") {
		t.Errorf("Location = %q, want to contain 'success=created'", location)
	}

	// Verify announcement was created
	announcements, err := annStore.List(ctx)
	if err != nil {
		t.Fatalf("failed to list announcements: %v", err)
	}
	if len(announcements) == 0 {
		t.Error("announcement should have been created")
	}
}

func TestCreate_MissingTitle(t *testing.T) {
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
	form.Set("title", "")
	form.Set("content", "Test content")
	form.Set("type", "info")

	req := httptest.NewRequest(http.MethodPost, "/announcements/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	// Should not redirect (should show error)
	if rec.Code == http.StatusSeeOther && strings.Contains(rec.Header().Get("Location"), "success") {
		t.Error("missing title should not succeed")
	}
}

func TestToggle_Success(t *testing.T) {
	h, _, annStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create an announcement
	ann, err := annStore.Create(ctx, announcement.CreateInput{
		Title:   "Toggle Test",
		Content: "Test content",
		Type:    announcement.TypeInfo,
		Active:  true,
	})
	if err != nil {
		t.Fatalf("failed to create announcement: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/announcements/"+ann.ID.Hex()+"/toggle", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", ann.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.toggle(rec, req)

	// Should redirect
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// Verify announcement was toggled
	toggled, err := annStore.GetByID(ctx, ann.ID)
	if err != nil {
		t.Fatalf("failed to get announcement: %v", err)
	}
	if toggled.Active == ann.Active {
		t.Error("announcement Active status should have changed")
	}
}

func TestDelete_Success(t *testing.T) {
	h, _, annStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create an announcement
	ann, err := annStore.Create(ctx, announcement.CreateInput{
		Title:   "Delete Test",
		Content: "Test content",
		Type:    announcement.TypeInfo,
		Active:  true,
	})
	if err != nil {
		t.Fatalf("failed to create announcement: %v", err)
	}

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/announcements/"+ann.ID.Hex()+"/delete", nil)
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", ann.ID.Hex())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.delete(rec, req)

	// Should redirect
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "success=deleted") {
		t.Errorf("Location = %q, want to contain 'success=deleted'", location)
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

	req := httptest.NewRequest(http.MethodGet, "/announcements/"+nonExistentID.Hex(), nil)
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

	req := httptest.NewRequest(http.MethodGet, "/announcements/"+nonExistentID.Hex()+"/manage_modal", nil)
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

func TestListVM(t *testing.T) {
	vm := ListVM{}
	vm.Title = "Announcements"
	vm.Success = "Test success"
	vm.Error = "Test error"
	vm.Items = []announcementRow{
		{ID: "test-id", Title: "Test Announcement", Type: announcement.TypeInfo},
	}

	if vm.Title != "Announcements" {
		t.Errorf("Title = %q, want %q", vm.Title, "Announcements")
	}
	if len(vm.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(vm.Items))
	}
}

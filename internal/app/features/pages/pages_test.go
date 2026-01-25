package pages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	pagestore "github.com/dalemusser/stratasave/internal/app/store/pages"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/stratasave/internal/testutil"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *mongo.Database, *pagestore.Store) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	handler := NewHandler(db, nil, logger)

	return handler, db, pagestore.New(db)
}

func TestNewHandler(t *testing.T) {
	h, _, _ := newTestHandler(t)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestAboutRouter(t *testing.T) {
	h, _, _ := newTestHandler(t)
	router := h.AboutRouter()
	if router == nil {
		t.Fatal("AboutRouter() returned nil")
	}
}

func TestContactRouter(t *testing.T) {
	h, _, _ := newTestHandler(t)
	router := h.ContactRouter()
	if router == nil {
		t.Fatal("ContactRouter() returned nil")
	}
}

func TestTermsRouter(t *testing.T) {
	h, _, _ := newTestHandler(t)
	router := h.TermsRouter()
	if router == nil {
		t.Fatal("TermsRouter() returned nil")
	}
}

func TestPrivacyRouter(t *testing.T) {
	h, _, _ := newTestHandler(t)
	router := h.PrivacyRouter()
	if router == nil {
		t.Fatal("PrivacyRouter() returned nil")
	}
}

func TestEditRoutes(t *testing.T) {
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

	router := EditRoutes(h, sessionMgr)
	if router == nil {
		t.Fatal("EditRoutes() returned nil")
	}
}

func TestShowPage_About(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	handler := h.showPage("about", "About")
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestShowPage_Contact(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/contact", nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	handler := h.showPage("contact", "Contact")
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestShowPage_Terms(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/terms", nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	handler := h.showPage("terms", "Terms of Service")
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestShowPage_Privacy(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/privacy", nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	handler := h.showPage("privacy", "Privacy Policy")
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestShowPage_AdminCanEdit(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	handler := h.showPage("about", "About")
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestListPages(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/pages", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.listPages(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEditPage(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/pages/about/edit", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "about")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.editPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUpdatePage_Success(t *testing.T) {
	h, _, pageStore := newTestHandler(t)
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
	form.Set("title", "Updated About Page")
	form.Set("content", "<p>Updated content</p>")

	req := httptest.NewRequest(http.MethodPost, "/pages/about", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "about")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.updatePage(rec, req)

	// Should redirect on success
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "success=1") {
		t.Errorf("Location = %q, want to contain 'success=1'", location)
	}

	// Verify page was saved
	page, err := pageStore.GetBySlug(ctx, "about")
	if err != nil {
		t.Fatalf("failed to get page: %v", err)
	}
	if page.Title != "Updated About Page" {
		t.Errorf("page.Title = %q, want %q", page.Title, "Updated About Page")
	}
}

func TestUpdatePage_TooLong(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	// Create content that exceeds MaxContentLength
	longContent := strings.Repeat("x", MaxContentLength+1)

	form := url.Values{}
	form.Set("title", "Test Page")
	form.Set("content", longContent)

	req := httptest.NewRequest(http.MethodPost, "/pages/about", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "about")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	h.updatePage(rec, req)

	// Should not redirect to success
	if rec.Code == http.StatusSeeOther && strings.Contains(rec.Header().Get("Location"), "success=1") {
		t.Error("content too long should not succeed")
	}
}

func TestPageDisplayName(t *testing.T) {
	tests := []struct {
		slug     string
		expected string
	}{
		{"about", "About"},
		{"contact", "Contact"},
		{"terms", "Terms of Service"},
		{"privacy", "Privacy Policy"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := pageDisplayName(tt.slug)
			if got != tt.expected {
				t.Errorf("pageDisplayName(%q) = %q, want %q", tt.slug, got, tt.expected)
			}
		})
	}
}

func TestPageVM(t *testing.T) {
	vm := PageVM{
		Slug:    "test",
		Content: "<p>Test content</p>",
		CanEdit: true,
	}
	vm.Title = "Test Page"

	if vm.Slug != "test" {
		t.Errorf("Slug = %q, want %q", vm.Slug, "test")
	}
	if vm.Title != "Test Page" {
		t.Errorf("Title = %q, want %q", vm.Title, "Test Page")
	}
	if !vm.CanEdit {
		t.Error("CanEdit should be true")
	}
}

func TestEditPageVM(t *testing.T) {
	vm := EditPageVM{
		Slug:      "about",
		PageTitle: "About Us",
		Content:   "<p>Content</p>",
		Success:   true,
		Error:     "",
	}
	vm.Title = "Edit Page"

	if vm.Slug != "about" {
		t.Errorf("Slug = %q, want %q", vm.Slug, "about")
	}
	if !vm.Success {
		t.Error("Success should be true")
	}
}

func TestShowPage_WithExistingContent(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _, pageStore := newTestHandler(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a page first
	page := models.Page{
		Slug:    "about",
		Title:   "About Us",
		Content: "<p>Existing content</p>",
	}
	err := pageStore.Upsert(ctx, page)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	handler := h.showPage("about", "About")
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

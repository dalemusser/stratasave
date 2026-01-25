// internal/app/features/pages/pages.go
package pages

import (
	"html/template"
	"net/http"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	pagestore "github.com/dalemusser/stratasave/internal/app/store/pages"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/htmlsanitize"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler provides page content handlers.
type Handler struct {
	pageStore *pagestore.Store
	errLog    *errorsfeature.ErrorLogger
	logger    *zap.Logger
}

// NewHandler creates a new pages Handler.
func NewHandler(db *mongo.Database, errLog *errorsfeature.ErrorLogger, logger *zap.Logger) *Handler {
	return &Handler{
		pageStore: pagestore.New(db),
		errLog:    errLog,
		logger:    logger,
	}
}

// PageVM is the view model for page content.
type PageVM struct {
	viewdata.BaseVM
	Slug    string
	Content template.HTML
	CanEdit bool
}

// AboutRouter returns a router for the about page.
func (h *Handler) AboutRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.showPage("about", "About"))
	return r
}

// ContactRouter returns a router for the contact page.
func (h *Handler) ContactRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.showPage("contact", "Contact"))
	return r
}

// TermsRouter returns a router for the terms page.
func (h *Handler) TermsRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.showPage("terms", "Terms of Service"))
	return r
}

// PrivacyRouter returns a router for the privacy page.
func (h *Handler) PrivacyRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.showPage("privacy", "Privacy Policy"))
	return r
}

// showPage returns a handler that displays a page by slug.
func (h *Handler) showPage(slug, defaultTitle string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := h.pageStore.GetBySlug(r.Context(), slug)
		if err != nil && err != mongo.ErrNoDocuments {
			h.errLog.Log(r, "failed to get page", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Check if user is admin for edit button
		canEdit := false
		if user, ok := auth.CurrentUser(r); ok && user.Role == "admin" {
			canEdit = true
		}

		vm := PageVM{
			BaseVM:  viewdata.New(r),
			Slug:    slug,
			CanEdit: canEdit,
		}
		vm.Title = defaultTitle

		if err == nil {
			vm.Title = page.Title
			vm.Content = htmlsanitize.PrepareForDisplay(page.Content)
		}

		templates.Render(w, r, "pages/show", vm)
	}
}

// EditRoutes returns routes for editing pages (admin only).
func EditRoutes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireRole("admin"))

	r.Get("/", h.listPages)
	r.Get("/{slug}/edit", h.editPage)
	r.Post("/{slug}", h.updatePage)

	return r
}

// EditPageVM is the view model for editing a page.
type EditPageVM struct {
	viewdata.BaseVM
	Slug      string
	PageTitle string
	Content   string
	Success   bool
	Error     string
}

// pageDisplayName returns a human-friendly name for a page slug.
func pageDisplayName(slug string) string {
	switch slug {
	case "about":
		return "About"
	case "contact":
		return "Contact"
	case "terms":
		return "Terms of Service"
	case "privacy":
		return "Privacy Policy"
	default:
		return slug
	}
}

// listPages shows all editable pages.
func (h *Handler) listPages(w http.ResponseWriter, r *http.Request) {
	pageSlugs := []string{"about", "contact", "terms", "privacy"}

	vm := struct {
		viewdata.BaseVM
		Pages []string
	}{
		BaseVM: viewdata.New(r),
		Pages:  pageSlugs,
	}
	vm.Title = "Manage Pages"

	templates.Render(w, r, "pages/list", vm)
}

// editPage shows the edit form for a page.
func (h *Handler) editPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	page, err := h.pageStore.GetBySlug(r.Context(), slug)
	if err != nil && err != mongo.ErrNoDocuments {
		h.errLog.Log(r, "failed to get page for edit", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	vm := EditPageVM{
		BaseVM: viewdata.New(r),
		Slug:   slug,
	}
	vm.Title = "Edit " + pageDisplayName(slug)

	// Check for success query parameter
	if r.URL.Query().Get("success") == "1" {
		vm.Success = true
	}

	if err == nil {
		vm.PageTitle = page.Title
		vm.Content = page.Content
	}

	templates.Render(w, r, "pages/edit", vm)
}

// MaxContentLength is the maximum allowed length for page content (100KB).
const MaxContentLength = 100000

// updatePage saves changes to a page.
func (h *Handler) updatePage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	rawContent := r.FormValue("content")

	// Validate content length before processing
	if len(rawContent) > MaxContentLength {
		vm := EditPageVM{
			BaseVM:    viewdata.New(r),
			Slug:      slug,
			PageTitle: title,
			Content:   rawContent,
			Error:     "Content is too long. Maximum length is 100,000 characters.",
		}
		vm.Title = "Edit " + pageDisplayName(slug)
		templates.Render(w, r, "pages/edit", vm)
		return
	}

	content := htmlsanitize.Sanitize(rawContent)

	page := models.Page{
		Slug:    slug,
		Title:   title,
		Content: content,
	}

	if err := h.pageStore.Upsert(r.Context(), page); err != nil {
		h.errLog.Log(r, "failed to update page", err)

		vm := EditPageVM{
			BaseVM:    viewdata.New(r),
			Slug:      slug,
			PageTitle: title,
			Content:   content,
			Error:     "Failed to save page. Please try again.",
		}
		vm.Title = "Edit " + pageDisplayName(slug)
		templates.Render(w, r, "pages/edit", vm)
		return
	}

	// Redirect back to edit page with success message
	http.Redirect(w, r, "/pages/"+slug+"/edit?success=1", http.StatusSeeOther)
}

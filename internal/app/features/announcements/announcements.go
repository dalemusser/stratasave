// internal/app/features/announcements/announcements.go
package announcements

import (
	"context"
	"net/http"
	"strings"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/announcement"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler provides announcement handlers.
type Handler struct {
	announcementStore *announcement.Store
	errLog            *errorsfeature.ErrorLogger
	logger            *zap.Logger
}

// NewHandler creates a new announcements Handler.
func NewHandler(
	db *mongo.Database,
	errLog *errorsfeature.ErrorLogger,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		announcementStore: announcement.New(db),
		errLog:            errLog,
		logger:            logger,
	}
}

// announcementRow represents an announcement in the list.
type announcementRow struct {
	ID          string
	Title       string
	Type        announcement.Type
	Active      bool
	Dismissible bool
	StartsAt    string
	EndsAt      string
}

// ListVM is the view model for the announcements list.
type ListVM struct {
	viewdata.BaseVM
	Items   []announcementRow // Named Items to avoid conflict with BaseVM.Announcements
	Success string
	Error   string
}

// Routes returns a chi.Router with announcement routes mounted.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireRole("admin"))

	r.Get("/", h.list)
	r.Get("/new", h.showNew)
	r.Post("/new", h.create)
	r.Get("/{id}", h.show)
	r.Get("/{id}/manage_modal", h.manageModal)
	r.Get("/{id}/edit", h.showEdit)
	r.Post("/{id}", h.update)
	r.Post("/{id}/toggle", h.toggle)
	r.Post("/{id}/delete", h.delete)

	return r
}

// list displays all announcements.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	announcements, err := h.announcementStore.List(r.Context())
	if err != nil {
		h.errLog.Log(r, "failed to list announcements", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rows := make([]announcementRow, 0, len(announcements))
	for _, ann := range announcements {
		startsAt := ""
		if ann.StartsAt != nil {
			startsAt = ann.StartsAt.Format("Jan 2, 2006 3:04 PM")
		}
		endsAt := ""
		if ann.EndsAt != nil {
			endsAt = ann.EndsAt.Format("Jan 2, 2006 3:04 PM")
		}
		rows = append(rows, announcementRow{
			ID:          ann.ID.Hex(),
			Title:       ann.Title,
			Type:        ann.Type,
			Active:      ann.Active,
			Dismissible: ann.Dismissible,
			StartsAt:    startsAt,
			EndsAt:      endsAt,
		})
	}

	vm := ListVM{
		BaseVM: viewdata.New(r),
		Items:  rows,
	}
	vm.Title = "Announcements"

	switch r.URL.Query().Get("success") {
	case "created":
		vm.Success = "Announcement created successfully"
	case "updated":
		vm.Success = "Announcement updated successfully"
	case "deleted":
		vm.Success = "Announcement deleted"
	case "toggled":
		vm.Success = "Announcement status updated"
	}

	templates.Render(w, r, "announcements/list", vm)
}

// NewVM is the view model for creating a new announcement.
type NewVM struct {
	viewdata.BaseVM
	AnnTitle    string // renamed to avoid conflict with BaseVM.Title
	Content     string
	Type        string
	Dismissible bool
	Active      bool
	StartsAt    string
	EndsAt      string
	Error       string
}

// showNew displays the new announcement form.
func (h *Handler) showNew(w http.ResponseWriter, r *http.Request) {
	vm := NewVM{
		BaseVM:      viewdata.New(r),
		Type:        "info",
		Dismissible: true,
		Active:      true,
	}
	vm.BaseVM.Title = "New Announcement"
	vm.BackURL = "/announcements"

	templates.Render(w, r, "announcements/new", vm)
}

// create creates a new announcement.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))
	annType := announcement.Type(r.FormValue("type"))
	dismissible := r.FormValue("dismissible") == "on"
	active := r.FormValue("active") == "on"

	if title == "" {
		vm := NewVM{
			BaseVM:      viewdata.New(r),
			AnnTitle:    title,
			Content:     content,
			Type:        string(annType),
			Dismissible: dismissible,
			Active:      active,
			Error:       "Title is required",
		}
		vm.BaseVM.Title = "New Announcement"
		vm.BackURL = "/announcements"
		templates.Render(w, r, "announcements/new", vm)
		return
	}

	input := announcement.CreateInput{
		Title:       title,
		Content:     content,
		Type:        annType,
		Dismissible: dismissible,
		Active:      active,
	}

	// Parse optional start/end times
	if startsAt := r.FormValue("starts_at"); startsAt != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", startsAt, time.Local); err == nil {
			input.StartsAt = &t
		}
	}
	if endsAt := r.FormValue("ends_at"); endsAt != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", endsAt, time.Local); err == nil {
			input.EndsAt = &t
		}
	}

	if _, err := h.announcementStore.Create(r.Context(), input); err != nil {
		h.errLog.Log(r, "failed to create announcement", err)
		vm := NewVM{
			BaseVM:      viewdata.New(r),
			AnnTitle:    title,
			Content:     content,
			Type:        string(annType),
			Dismissible: dismissible,
			Active:      active,
			Error:       "Failed to create announcement",
		}
		vm.BaseVM.Title = "New Announcement"
		vm.BackURL = "/announcements"
		templates.Render(w, r, "announcements/new", vm)
		return
	}

	http.Redirect(w, r, "/announcements?success=created", http.StatusSeeOther)
}

// EditVM is the view model for editing an announcement.
type EditVM struct {
	viewdata.BaseVM
	ID          string
	AnnTitle    string // renamed to avoid conflict with BaseVM.Title
	Content     string
	Type        string
	Dismissible bool
	Active      bool
	StartsAt    string
	EndsAt      string
	Error       string
}

// ManageModalVM is the view model for the manage modal.
type ManageModalVM struct {
	ID        string
	Title     string
	Type      string
	Active    bool
	BackURL   string
	CSRFToken string
}

// ShowVM is the view model for viewing an announcement.
type ShowVM struct {
	viewdata.BaseVM
	ID          string
	AnnTitle    string
	Content     string
	Type        string
	Dismissible bool
	Active      bool
	StartsAt    string
	EndsAt      string
}

// show displays a single announcement.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ann, err := h.announcementStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	backURL := r.URL.Query().Get("return")
	if backURL == "" {
		backURL = "/announcements"
	}

	startsAt := ""
	if ann.StartsAt != nil {
		startsAt = ann.StartsAt.Format("Jan 2, 2006 3:04 PM")
	}
	endsAt := ""
	if ann.EndsAt != nil {
		endsAt = ann.EndsAt.Format("Jan 2, 2006 3:04 PM")
	}

	vm := ShowVM{
		BaseVM:      viewdata.New(r),
		ID:          id,
		AnnTitle:    ann.Title,
		Content:     ann.Content,
		Type:        string(ann.Type),
		Dismissible: ann.Dismissible,
		Active:      ann.Active,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
	}
	vm.Title = "View Announcement"
	vm.BackURL = backURL

	templates.Render(w, r, "announcements/show", vm)
}

// manageModal displays the manage modal for an announcement.
func (h *Handler) manageModal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ann, err := h.announcementStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	backURL := r.URL.Query().Get("return")
	if backURL == "" {
		backURL = "/announcements"
	}

	vm := ManageModalVM{
		ID:        id,
		Title:     ann.Title,
		Type:      string(ann.Type),
		Active:    ann.Active,
		BackURL:   backURL,
		CSRFToken: csrf.Token(r),
	}

	templates.RenderSnippet(w, "announcements/manage_modal", vm)
}

// showEdit displays the edit announcement form.
func (h *Handler) showEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ann, err := h.announcementStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	startsAt := ""
	if ann.StartsAt != nil {
		startsAt = ann.StartsAt.Format("2006-01-02T15:04")
	}
	endsAt := ""
	if ann.EndsAt != nil {
		endsAt = ann.EndsAt.Format("2006-01-02T15:04")
	}

	vm := EditVM{
		BaseVM:      viewdata.New(r),
		ID:          id,
		AnnTitle:    ann.Title,
		Content:     ann.Content,
		Type:        string(ann.Type),
		Dismissible: ann.Dismissible,
		Active:      ann.Active,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
	}
	vm.Title = "Edit Announcement"
	vm.BackURL = "/announcements"

	templates.Render(w, r, "announcements/edit", vm)
}

// update updates an announcement.
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))
	annType := announcement.Type(r.FormValue("type"))
	dismissible := r.FormValue("dismissible") == "on"
	active := r.FormValue("active") == "on"

	if title == "" {
		vm := EditVM{
			BaseVM:      viewdata.New(r),
			ID:          id,
			AnnTitle:    title,
			Content:     content,
			Type:        string(annType),
			Dismissible: dismissible,
			Active:      active,
			Error:       "Title is required",
		}
		vm.BackURL = "/announcements"
		templates.Render(w, r, "announcements/edit", vm)
		return
	}

	input := announcement.UpdateInput{
		Title:       &title,
		Content:     &content,
		Type:        &annType,
		Dismissible: &dismissible,
		Active:      &active,
	}

	// Parse optional start/end times
	if startsAt := r.FormValue("starts_at"); startsAt != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", startsAt, time.Local); err == nil {
			input.StartsAt = &t
		}
	}
	if endsAt := r.FormValue("ends_at"); endsAt != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", endsAt, time.Local); err == nil {
			input.EndsAt = &t
		}
	}

	if err := h.announcementStore.Update(r.Context(), objID, input); err != nil {
		h.errLog.Log(r, "failed to update announcement", err)
		vm := EditVM{
			BaseVM:      viewdata.New(r),
			ID:          id,
			AnnTitle:    title,
			Content:     content,
			Type:        string(annType),
			Dismissible: dismissible,
			Active:      active,
			Error:       "Failed to update announcement",
		}
		vm.BackURL = "/announcements"
		templates.Render(w, r, "announcements/edit", vm)
		return
	}

	http.Redirect(w, r, "/announcements?success=updated", http.StatusSeeOther)
}

// toggle toggles the active status of an announcement.
func (h *Handler) toggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ann, err := h.announcementStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.announcementStore.SetActive(r.Context(), objID, !ann.Active); err != nil {
		h.errLog.Log(r, "failed to toggle announcement", err)
		http.Redirect(w, r, "/announcements?error=toggle_failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/announcements?success=toggled", http.StatusSeeOther)
}

// delete deletes an announcement.
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.announcementStore.Delete(r.Context(), objID); err != nil {
		h.errLog.Log(r, "failed to delete announcement", err)
		http.Redirect(w, r, "/announcements?error=delete_failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/announcements?success=deleted", http.StatusSeeOther)
}

// GetActiveAnnouncements returns active announcements for display in the UI.
func (h *Handler) GetActiveAnnouncements(ctx context.Context) ([]announcement.Announcement, error) {
	return h.announcementStore.GetActive(ctx)
}

// GetStore returns the underlying announcement store for use by other components.
func (h *Handler) GetStore() *announcement.Store {
	return h.announcementStore
}

// ViewVM is the view model for the user-facing announcements view.
type ViewVM struct {
	viewdata.BaseVM
	Items []viewAnnouncementRow
}

// viewAnnouncementRow represents an announcement in the user view.
type viewAnnouncementRow struct {
	ID          string
	Title       string
	Content     string
	Type        string // info, warning, critical
	Dismissible bool
}

// ViewRoutes returns routes for the user-facing announcements view.
// These routes require authentication but not admin role.
func ViewRoutes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireAuth)

	r.Get("/", h.viewAnnouncements)

	return r
}

// viewAnnouncements displays all active announcements for the user.
func (h *Handler) viewAnnouncements(w http.ResponseWriter, r *http.Request) {
	announcements, err := h.announcementStore.GetActive(r.Context())
	if err != nil {
		h.errLog.Log(r, "failed to get active announcements", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rows := make([]viewAnnouncementRow, 0, len(announcements))
	for _, ann := range announcements {
		rows = append(rows, viewAnnouncementRow{
			ID:          ann.ID.Hex(),
			Title:       ann.Title,
			Content:     ann.Content,
			Type:        string(ann.Type),
			Dismissible: ann.Dismissible,
		})
	}

	vm := ViewVM{
		BaseVM: viewdata.New(r),
		Items:  rows,
	}
	vm.Title = "Announcements"
	vm.BackURL = "/dashboard"

	templates.Render(w, r, "announcements/view", vm)
}

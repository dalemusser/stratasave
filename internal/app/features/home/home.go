// internal/app/features/home/home.go
package home

import (
	"html/template"
	"net/http"

	settingsstore "github.com/dalemusser/stratasave/internal/app/store/settings"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/htmlsanitize"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler provides home page handlers.
type Handler struct {
	db     *mongo.Database
	logger *zap.Logger
}

// NewHandler creates a new home Handler.
func NewHandler(db *mongo.Database, logger *zap.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger,
	}
}

// HomeVM is the view model for the home page.
type HomeVM struct {
	viewdata.BaseVM
	LandingTitle string        // Title for landing page
	Content      template.HTML // Landing page content (HTML)
	CanEdit      bool          // True if user can edit the landing page
}

// Routes returns a chi.Router with home routes mounted.
func Routes(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.Index)
	return r
}

// Index renders the home page.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	vm := HomeVM{
		BaseVM: viewdata.New(r),
	}
	vm.Title = "Home"

	// Check if user can edit (admin role)
	if user, ok := auth.CurrentUser(r); ok && user.Role == "admin" {
		vm.CanEdit = true
	}

	// Get landing page title and content from settings
	store := settingsstore.New(h.db)
	settings, err := store.Get(r.Context())
	if err != nil {
		h.logger.Warn("failed to load settings for landing page", zap.Error(err))
		vm.LandingTitle = models.DefaultLandingTitle
		vm.Content = htmlsanitize.SanitizeToHTML(models.DefaultLandingContent)
	} else {
		// Settings store returns defaults if no document exists
		vm.LandingTitle = settings.LandingTitle
		if vm.LandingTitle == "" {
			vm.LandingTitle = models.DefaultLandingTitle
		}
		if settings.LandingContent == "" {
			vm.Content = htmlsanitize.SanitizeToHTML(models.DefaultLandingContent)
		} else {
			vm.Content = htmlsanitize.SanitizeToHTML(settings.LandingContent)
		}
	}

	templates.Render(w, r, "home/index", vm)
}

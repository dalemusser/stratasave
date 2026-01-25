// internal/app/features/settings/settings.go
package settings

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	settingsstore "github.com/dalemusser/stratasave/internal/app/store/settings"
	"github.com/dalemusser/stratasave/internal/app/system/htmlsanitize"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/storage"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler provides settings handlers.
type Handler struct {
	settingsStore *settingsstore.Store
	fileStorage   storage.Store
	errLog        *errorsfeature.ErrorLogger
	logger        *zap.Logger
}

// NewHandler creates a new settings Handler.
func NewHandler(
	db *mongo.Database,
	fileStorage storage.Store,
	errLog *errorsfeature.ErrorLogger,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		settingsStore: settingsstore.New(db),
		fileStorage:   fileStorage,
		errLog:        errLog,
		logger:        logger,
	}
}

// SettingsVM is the view model for the settings page.
type SettingsVM struct {
	viewdata.BaseVM
	Settings       *models.SiteSettings
	LandingTitle   string // Landing page title (with default if empty)
	LandingContent string // Landing page content
	HasLogo        bool   // Whether a logo is uploaded
	LogoURL        string // Generated URL for the logo
	LogoName       string // Original filename of the logo
	Success        string
	Error          string
}

// MountRoutes mounts settings routes on the given router.
func (h *Handler) MountRoutes(r chi.Router) {
	r.Get("/", h.show)
	r.Post("/", h.update)
}

// show displays the settings page.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsStore.Get(r.Context())
	if err != nil && err != mongo.ErrNoDocuments {
		h.errLog.Log(r, "failed to get settings", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if settings == nil {
		settings = &models.SiteSettings{
			SiteName: "Strata",
		}
	}

	// Use default landing title if empty so admin has something to work with
	landingTitle := settings.LandingTitle
	if landingTitle == "" {
		landingTitle = models.DefaultLandingTitle
	}

	// Generate logo URL if exists
	var logoURL string
	if settings.HasLogo() {
		logoURL = h.fileStorage.URL(settings.LogoPath)
	}

	vm := SettingsVM{
		BaseVM:         viewdata.New(r),
		Settings:       settings,
		LandingTitle:   landingTitle,
		LandingContent: settings.LandingContent,
		HasLogo:        settings.HasLogo(),
		LogoURL:        logoURL,
		LogoName:       settings.LogoName,
	}
	vm.Title = "Site Settings"
	vm.SiteName = settings.SiteName
	vm.FooterHTML = htmlsanitize.SanitizeToHTML(settings.FooterHTML)

	if r.URL.Query().Get("success") == "1" {
		vm.Success = "Settings updated successfully"
	}

	templates.Render(w, r, "settings/show", vm)
}

// MaxContentLength is the maximum allowed length for HTML content fields (100KB).
const MaxContentLength = 100000

// MaxFooterLength is the maximum allowed length for footer HTML (10KB).
const MaxFooterLength = 10000

// update saves the settings including logo handling.
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form for file uploads (10MB max)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	siteName := r.FormValue("site_name")
	landingTitle := r.FormValue("landing_title")
	rawLandingContent := r.FormValue("landing_content")
	rawFooterHTML := r.FormValue("footer_html")
	removeLogo := r.FormValue("remove_logo") != ""

	// Validate content lengths
	if len(rawLandingContent) > MaxContentLength {
		h.renderSettingsWithError(w, r, "Landing content is too long. Maximum length is 100,000 characters.")
		return
	}
	if len(rawFooterHTML) > MaxFooterLength {
		h.renderSettingsWithError(w, r, "Footer HTML is too long. Maximum length is 10,000 characters.")
		return
	}

	landingContent := htmlsanitize.Sanitize(rawLandingContent)
	footerHTML := htmlsanitize.Sanitize(rawFooterHTML)

	// Get current settings for logo handling
	current, _ := h.settingsStore.Get(ctx)
	if current == nil {
		current = &models.SiteSettings{}
	}

	// Handle logo upload/removal
	logoPath := current.LogoPath
	logoName := current.LogoName

	if removeLogo {
		// Delete old logo if exists
		if current.HasLogo() {
			if err := h.fileStorage.Delete(ctx, current.LogoPath); err != nil {
				h.logger.Warn("failed to delete old logo", zap.String("path", current.LogoPath), zap.Error(err))
			}
		}
		logoPath = ""
		logoName = ""
	}

	// Check for new logo upload
	file, header, fileErr := r.FormFile("logo")
	hasNewLogo := fileErr == nil && header != nil && header.Size > 0
	if hasNewLogo {
		defer file.Close()

		// Delete old logo if exists
		if current.HasLogo() {
			if err := h.fileStorage.Delete(ctx, current.LogoPath); err != nil {
				h.logger.Warn("failed to delete old logo", zap.String("path", current.LogoPath), zap.Error(err))
			}
		}

		// Upload new logo with unique path
		newPath, err := h.uploadLogoFile(ctx, header.Filename, file, header.Header.Get("Content-Type"))
		if err != nil {
			h.logger.Error("logo upload failed", zap.Error(err))
			h.renderSettingsWithError(w, r, "Failed to upload logo. Please try again.")
			return
		}
		logoPath = newPath
		logoName = header.Filename
	}

	// Parse email notification settings (checkboxes)
	notifyUserOnCreate := r.FormValue("notify_user_on_create") == "on"
	notifyUserOnDisable := r.FormValue("notify_user_on_disable") == "on"
	notifyUserOnEnable := r.FormValue("notify_user_on_enable") == "on"
	notifyUserOnWelcome := r.FormValue("notify_user_on_welcome") == "on"

	input := settingsstore.UpdateInput{
		SiteName:            siteName,
		LandingTitle:        landingTitle,
		LandingContent:      landingContent,
		FooterHTML:          footerHTML,
		LogoPath:            logoPath,
		LogoName:            logoName,
		NotifyUserOnCreate:  notifyUserOnCreate,
		NotifyUserOnDisable: notifyUserOnDisable,
		NotifyUserOnEnable:  notifyUserOnEnable,
		NotifyUserOnWelcome: notifyUserOnWelcome,
	}

	if err := h.settingsStore.Upsert(ctx, input); err != nil {
		h.errLog.Log(r, "failed to update settings", err)
		h.renderSettingsWithError(w, r, "Failed to save settings")
		return
	}

	http.Redirect(w, r, "/settings?success=1", http.StatusSeeOther)
}

// renderSettingsWithError re-renders the settings page with an error message.
func (h *Handler) renderSettingsWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	settings, _ := h.settingsStore.Get(r.Context())
	if settings == nil {
		settings = &models.SiteSettings{SiteName: "Strata"}
	}

	landingTitle := settings.LandingTitle
	if landingTitle == "" {
		landingTitle = models.DefaultLandingTitle
	}

	var logoURL string
	if settings.HasLogo() {
		logoURL = h.fileStorage.URL(settings.LogoPath)
	}

	vm := SettingsVM{
		BaseVM:         viewdata.New(r),
		Settings:       settings,
		LandingTitle:   landingTitle,
		LandingContent: settings.LandingContent,
		HasLogo:        settings.HasLogo(),
		LogoURL:        logoURL,
		LogoName:       settings.LogoName,
		Error:          errMsg,
	}
	vm.Title = "Site Settings"
	vm.SiteName = settings.SiteName
	vm.FooterHTML = htmlsanitize.SanitizeToHTML(settings.FooterHTML)

	templates.Render(w, r, "settings/show", vm)
}

// uploadLogoFile stores a logo file with a unique path and returns the storage path.
func (h *Handler) uploadLogoFile(ctx context.Context, filename string, file io.Reader, contentType string) (string, error) {
	// Generate unique path: logos/YYYY/MM/uuid-ext
	now := time.Now().UTC()
	ext := filepath.Ext(filename)
	uniqueName := fmt.Sprintf("%s%s", uuid.New().String()[:8], ext)
	path := fmt.Sprintf("logos/%04d/%02d/%s", now.Year(), now.Month(), uniqueName)

	opts := &storage.PutOptions{
		ContentType: contentType,
	}
	if err := h.fileStorage.Put(ctx, path, file, opts); err != nil {
		return "", fmt.Errorf("failed to upload logo: %w", err)
	}

	return path, nil
}

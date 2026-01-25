// internal/app/system/viewdata/viewdata.go
package viewdata

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"html/template"
	"net/http"

	settingsstore "github.com/dalemusser/stratasave/internal/app/store/settings"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/authz"
	"github.com/dalemusser/stratasave/internal/app/system/htmlsanitize"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/httpnav"
	"github.com/dalemusser/waffle/pantry/storage"
	"github.com/gorilla/csrf"
	"go.mongodb.org/mongo-driver/mongo"
)

// AnnouncementVM represents an announcement for display in templates.
type AnnouncementVM struct {
	ID          string
	Title       string
	Content     string
	Type        string // info, warning, critical
	Dismissible bool
}

// BaseVM contains common fields for all view models.
// Embed this struct in your feature-specific view models.
//
// Usage:
//
//	type myPageData struct {
//	    viewdata.BaseVM
//	    // page-specific fields...
//	}
//
//	data := myPageData{
//	    BaseVM: viewdata.NewBaseVM(r, db, "Page Title", "/default-back"),
//	    // page-specific fields...
//	}
type BaseVM struct {
	// Site settings (from database)
	SiteName   string
	LogoURL    string
	FooterHTML template.HTML

	// User context (from auth middleware)
	IsLoggedIn      bool
	UserID          string
	LoginID         string // User's login identifier (for per-user tracking)
	Role            string
	UserName        string
	ThemePreference string // light, dark, system (empty = system)

	// Page context
	Title       string
	BackURL     string
	CurrentPath string

	// Security
	CSRFToken string // CSRF token for forms (use in hidden input field)

	// Announcements for banner display
	Announcements []AnnouncementVM
}

// storageProvider is set by Init and used to generate logo URLs.
var storageProvider storage.Store

// globalDB is set by Init and used by New() to load settings.
var globalDB *mongo.Database

// AnnouncementLoader is a function that loads active announcements.
// This is set by bootstrap to avoid circular dependencies.
type AnnouncementLoader func(ctx context.Context) []AnnouncementVM

var announcementLoader AnnouncementLoader

// Init sets the storage provider and database for viewdata.
// Call this once at startup from bootstrap.
func Init(store storage.Store, db *mongo.Database) {
	storageProvider = store
	globalDB = db
}

// SetAnnouncementLoader sets the function used to load active announcements.
// Call this once at startup from bootstrap after the announcement store is available.
func SetAnnouncementLoader(loader AnnouncementLoader) {
	announcementLoader = loader
}

// NewBaseVM creates a fully populated BaseVM for a page.
// This is the preferred way to create a BaseVM for embedding in view models.
//
// Parameters:
//   - r: the HTTP request
//   - db: database for loading site settings (can be nil for defaults)
//   - title: the page title
//   - backDefault: default URL for the back button if none in request
func NewBaseVM(r *http.Request, db *mongo.Database, title, backDefault string) BaseVM {
	role, name, userID, signedIn := authz.UserCtx(r)

	vm := BaseVM{
		SiteName:        models.DefaultSiteName,
		IsLoggedIn:      signedIn,
		UserID:          userID.Hex(),
		Role:            role,
		UserName:        name,
		ThemePreference: authz.ThemePreference(r),
		Title:           title,
		BackURL:         httpnav.ResolveBackURL(r, backDefault),
		CurrentPath:     httpnav.CurrentPath(r),
		CSRFToken:       csrf.Token(r),
	}

	// Get LoginID from session if logged in
	if signedIn {
		if user, ok := auth.CurrentUser(r); ok {
			vm.LoginID = user.LoginID
		}
	}

	if db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
		defer cancel()

		store := settingsstore.New(db)
		settings, err := store.Get(ctx)
		if err == nil && settings != nil {
			vm.SiteName = settings.SiteName
			footerHTML := settings.FooterHTML
			if footerHTML == "" {
				footerHTML = models.DefaultFooterHTML
			}
			vm.FooterHTML = htmlsanitize.SanitizeToHTML(footerHTML)
			if settings.HasLogo() && storageProvider != nil {
				vm.LogoURL = storageProvider.URL(settings.LogoPath)
			}
		}
	}

	// Load active announcements only if logged in and loader is configured
	if signedIn && announcementLoader != nil {
		vm.Announcements = announcementLoader(r.Context())
	}

	return vm
}

// GetSiteName returns the site name from settings, or the default if not available.
func GetSiteName(ctx context.Context, db *mongo.Database) string {
	if db == nil {
		return models.DefaultSiteName
	}

	store := settingsstore.New(db)
	settings, err := store.Get(ctx)
	if err != nil || settings == nil {
		return models.DefaultSiteName
	}
	return settings.SiteName
}

// GetSettings returns the full site settings, or defaults if not available.
func GetSettings(ctx context.Context, db *mongo.Database) models.SiteSettings {
	if db == nil {
		return models.SiteSettings{SiteName: models.DefaultSiteName}
	}

	store := settingsstore.New(db)
	settings, err := store.Get(ctx)
	if err != nil || settings == nil {
		return models.SiteSettings{SiteName: models.DefaultSiteName}
	}
	return *settings
}

// New creates a BaseVM with site settings loaded from the database.
// This is the standard way to create a BaseVM for most handlers.
func New(r *http.Request) BaseVM {
	role, name, userID, signedIn := authz.UserCtx(r)

	vm := BaseVM{
		SiteName:        models.DefaultSiteName,
		IsLoggedIn:      signedIn,
		UserID:          userID.Hex(),
		Role:            role,
		UserName:        name,
		ThemePreference: authz.ThemePreference(r),
		CurrentPath:     httpnav.CurrentPath(r),
		CSRFToken:       csrf.Token(r),
	}

	// Get LoginID from session if logged in
	if signedIn {
		if user, ok := auth.CurrentUser(r); ok {
			vm.LoginID = user.LoginID
		}
	}

	// Load site settings if database is available
	if globalDB != nil {
		ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
		defer cancel()

		store := settingsstore.New(globalDB)
		settings, err := store.Get(ctx)
		if err == nil && settings != nil {
			vm.SiteName = settings.SiteName
			footerHTML := settings.FooterHTML
			if footerHTML == "" {
				footerHTML = models.DefaultFooterHTML
			}
			vm.FooterHTML = htmlsanitize.SanitizeToHTML(footerHTML)
			if settings.HasLogo() && storageProvider != nil {
				vm.LogoURL = storageProvider.URL(settings.LogoPath)
			}
		}
	}

	// Load active announcements only if logged in and loader is configured
	if signedIn && announcementLoader != nil {
		vm.Announcements = announcementLoader(r.Context())
	}

	return vm
}

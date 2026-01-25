// internal/app/features/profile/profile.go
package profile

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/authutil"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler provides profile handlers.
type Handler struct {
	userStore     *userstore.Store
	sessionsStore *sessions.Store
	errLog        *errorsfeature.ErrorLogger
	logger        *zap.Logger
}

// NewHandler creates a new profile Handler.
func NewHandler(db *mongo.Database, sessionsStore *sessions.Store, errLog *errorsfeature.ErrorLogger, logger *zap.Logger) *Handler {
	return &Handler{
		userStore:     userstore.New(db),
		sessionsStore: sessionsStore,
		errLog:        errLog,
		logger:        logger,
	}
}

// ProfileVM is the view model for the profile page.
type ProfileVM struct {
	viewdata.BaseVM

	// User info (read-only display)
	FullName   string
	AuthMethod string

	// Password section (only shown for password auth)
	ShowPasswordSection bool
	PasswordRules       string

	// Preferences
	ThemePreference string // "light", "dark", "system"

	// Active sessions
	Sessions []sessionRow

	// Form state
	Success template.HTML
	Error   template.HTML
}

// Routes returns a chi.Router with profile routes mounted.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()

	r.Get("/", h.showProfile)
	r.Post("/password", h.handleChangePassword)
	r.Post("/preferences", h.handleUpdatePreferences)

	// Session management (sessions are now embedded in profile page)
	r.Get("/sessions", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	})
	r.Post("/sessions/{id}/revoke", h.revokeSession)
	r.Post("/sessions/revoke-all", h.revokeAllSessions(sessionMgr))

	// Legacy change password page (redirect to profile)
	r.Get("/change-password", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	})

	return r
}

// showProfile displays the user profile.
func (h *Handler) showProfile(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := auth.CurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), sessionUser.UserID())
	if err != nil {
		h.errLog.Log(r, "failed to get user", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Load active sessions
	sessionsList, err := h.sessionsStore.ListByUser(r.Context(), sessionUser.UserID())
	if err != nil {
		h.errLog.Log(r, "failed to list sessions", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	currentToken := sessionUser.SessionToken()
	sessionRows := make([]sessionRow, 0, len(sessionsList))
	for _, s := range sessionsList {
		sessionRows = append(sessionRows, sessionRow{
			ID:           s.ID.Hex(),
			IPAddress:    s.IPAddress,
			UserAgent:    s.UserAgent,
			Device:       parseDevice(s.UserAgent),
			LastActivity: s.LastActivity,
			IsCurrent:    s.Token == currentToken,
		})
	}

	vm := buildProfileVM(r, user)
	vm.Sessions = sessionRows

	// Check for success message in query params
	switch r.URL.Query().Get("success") {
	case "password":
		vm.Success = "Password changed successfully."
	case "preferences":
		vm.Success = "Preferences saved."
	case "revoked":
		vm.Success = "Session revoked successfully."
	case "revoked_all":
		vm.Success = "All other sessions have been logged out."
	}

	// Check for error message in query params
	switch r.URL.Query().Get("error") {
	case "use_logout":
		vm.Error = "Use the logout option to end your current session."
	case "failed":
		vm.Error = "Failed to revoke session. Please try again."
	}

	templates.Render(w, r, "profile/show", vm)
}

// handleChangePassword processes the password change form.
func (h *Handler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := auth.CurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), sessionUser.UserID())
	if err != nil {
		h.errLog.Log(r, "failed to get user", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Only allow password change for password auth users
	if user.AuthMethod != "password" {
		renderProfileWithError(w, r, user, "Password change is only available for password authentication.")
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Verify current password (skip if temp password)
	if user.PasswordHash != nil && (user.PasswordTemp == nil || !*user.PasswordTemp) {
		if !authutil.CheckPassword(currentPassword, *user.PasswordHash) {
			renderProfileWithError(w, r, user, "Current password is incorrect.")
			return
		}
	}

	// Validate new password
	if err := authutil.ValidatePassword(newPassword); err != nil {
		renderProfileWithError(w, r, user, err.Error())
		return
	}

	// Check passwords match
	if newPassword != confirmPassword {
		renderProfileWithError(w, r, user, "New passwords do not match.")
		return
	}

	// Don't allow reusing the current password
	if user.PasswordHash != nil && authutil.CheckPassword(newPassword, *user.PasswordHash) {
		renderProfileWithError(w, r, user, "New password cannot be the same as your current password.")
		return
	}

	// Hash and save the new password
	hash, err := authutil.HashPassword(newPassword)
	if err != nil {
		h.errLog.Log(r, "failed to hash password", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tempFalse := false
	if err := h.userStore.UpdateFromInput(r.Context(), sessionUser.UserID(), userstore.UpdateInput{
		PasswordHash: &hash,
		PasswordTemp: &tempFalse,
	}); err != nil {
		h.errLog.Log(r, "failed to update password", err)
		renderProfileWithError(w, r, user, "Failed to update password.")
		return
	}

	http.Redirect(w, r, "/profile?success=password", http.StatusSeeOther)
}

// handleUpdatePreferences processes the preferences form.
func (h *Handler) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := auth.CurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	theme := strings.TrimSpace(r.FormValue("theme_preference"))

	// Validate theme value
	switch theme {
	case "light", "dark", "system":
		// valid
	default:
		theme = "system"
	}

	if err := h.userStore.UpdateThemePreference(r.Context(), sessionUser.UserID(), theme); err != nil {
		h.errLog.Log(r, "failed to update theme preference", err)

		user, _ := h.userStore.GetByID(r.Context(), sessionUser.UserID())
		renderProfileWithError(w, r, user, "Failed to save preferences.")
		return
	}

	// Set theme preference cookie so the new theme applies immediately on redirect
	// HttpOnly is false to allow client-side JavaScript to read it for immediate theme application
	// MaxAge is 1 year (the database is the source of truth, this is just for client-side convenience)
	http.SetCookie(w, &http.Cookie{
		Name:     "theme_pref",
		Value:    theme,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60, // 1 year
		HttpOnly: false,              // Intentionally false for JS access to prevent theme flashing
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/profile?success=preferences", http.StatusSeeOther)
}

// buildProfileVM creates the profile view model from a user.
func buildProfileVM(r *http.Request, user *models.User) ProfileVM {
	themePreference := user.ThemePreference
	if themePreference == "" {
		themePreference = "system"
	}

	return ProfileVM{
		BaseVM:              viewdata.New(r),
		FullName:            user.FullName,
		AuthMethod:          formatAuthMethod(user.AuthMethod),
		ShowPasswordSection: user.AuthMethod == "password",
		PasswordRules:       authutil.PasswordRules(),
		ThemePreference:     themePreference,
	}
}

// renderProfileWithError re-renders the profile page with an error message.
func renderProfileWithError(w http.ResponseWriter, r *http.Request, user *models.User, errMsg string) {
	vm := buildProfileVM(r, user)
	vm.Error = template.HTML(errMsg)
	templates.Render(w, r, "profile/show", vm)
}

// formatAuthMethod returns a human-readable label for the auth method.
func formatAuthMethod(method string) string {
	switch method {
	case "password":
		return "Password"
	case "email":
		return "Email"
	case "google":
		return "Google"
	case "trust":
		return "Trusted"
	default:
		return method
	}
}

// sessionRow represents a session in the list.
type sessionRow struct {
	ID           string
	IPAddress    string
	UserAgent    string
	Device       string
	LastActivity time.Time
	IsCurrent    bool
}

// revokeSession revokes a specific session.
func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := auth.CurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get the session to verify ownership
	session, err := h.sessionsStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Verify the session belongs to the current user
	if session.UserID != sessionUser.UserID() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Don't allow revoking the current session via this endpoint
	if session.Token == sessionUser.SessionToken() {
		http.Redirect(w, r, "/profile?error=use_logout", http.StatusSeeOther)
		return
	}

	if err := h.sessionsStore.DeleteByID(r.Context(), objID); err != nil {
		h.errLog.Log(r, "failed to revoke session", err)
		http.Redirect(w, r, "/profile?error=failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profile?success=revoked", http.StatusSeeOther)
}

// revokeAllSessions returns a handler that revokes all sessions except the current one.
func (h *Handler) revokeAllSessions(sessionMgr *auth.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionUser, ok := auth.CurrentUser(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		currentToken := sessionUser.SessionToken()
		if err := h.sessionsStore.DeleteByUserExcept(r.Context(), sessionUser.UserID(), currentToken); err != nil {
			h.errLog.Log(r, "failed to revoke all sessions", err)
			http.Redirect(w, r, "/profile?error=failed", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/profile?success=revoked_all", http.StatusSeeOther)
	}
}

// parseDevice extracts a simple device description from the user agent string.
func parseDevice(userAgent string) string {
	if userAgent == "" {
		return "Unknown Device"
	}

	ua := strings.ToLower(userAgent)

	// Detect mobile devices
	if strings.Contains(ua, "iphone") {
		return "iPhone"
	}
	if strings.Contains(ua, "ipad") {
		return "iPad"
	}
	if strings.Contains(ua, "android") {
		if strings.Contains(ua, "mobile") {
			return "Android Phone"
		}
		return "Android Tablet"
	}

	// Detect browsers on desktop
	if strings.Contains(ua, "windows") {
		if strings.Contains(ua, "edge") {
			return "Windows (Edge)"
		}
		if strings.Contains(ua, "chrome") {
			return "Windows (Chrome)"
		}
		if strings.Contains(ua, "firefox") {
			return "Windows (Firefox)"
		}
		return "Windows"
	}

	if strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os") {
		if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
			return "Mac (Safari)"
		}
		if strings.Contains(ua, "chrome") {
			return "Mac (Chrome)"
		}
		if strings.Contains(ua, "firefox") {
			return "Mac (Firefox)"
		}
		return "Mac"
	}

	if strings.Contains(ua, "linux") {
		if strings.Contains(ua, "chrome") {
			return "Linux (Chrome)"
		}
		if strings.Contains(ua, "firefox") {
			return "Linux (Firefox)"
		}
		return "Linux"
	}

	return "Unknown Device"
}

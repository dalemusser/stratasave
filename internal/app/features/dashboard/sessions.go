// internal/app/features/dashboard/sessions.go
package dashboard

import (
	"net/http"
	"strconv"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// SessionsHandler handles the active sessions dashboard.
type SessionsHandler struct {
	db        *mongo.Database
	sessions  *sessions.Store
	userStore *userstore.Store
	logger    *zap.Logger
}

// NewSessionsHandler creates a new sessions handler.
func NewSessionsHandler(db *mongo.Database, sessionsStore *sessions.Store, logger *zap.Logger) *SessionsHandler {
	return &SessionsHandler{
		db:        db,
		sessions:  sessionsStore,
		userStore: userstore.New(db),
		logger:    logger,
	}
}

// SessionsRoutes returns routes for the sessions dashboard.
func SessionsRoutes(h *SessionsHandler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireRole("admin"))
	r.Get("/", h.listSessions)
	r.Get("/table", h.listSessionsTable)
	r.Post("/{id}/terminate", h.terminateSession)
	return r
}

// SessionVM represents a session in the view.
type SessionVM struct {
	ID               string
	UserID           string
	UserName         string
	UserEmail        string
	CurrentPage      string
	LastActivity     time.Time
	LastActivityAgo  string
	IPAddress        string
	DeviceInfo       string
	LoginAt          time.Time
	LoginAtFormatted string
	IsCurrentSession bool
}

// SessionsListVM is the view model for the sessions list.
type SessionsListVM struct {
	viewdata.BaseVM
	Sessions     []SessionVM
	CurrentToken string
}

// listSessions displays all active sessions.
func (h *SessionsHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current user's session token
	currentUser, _ := auth.CurrentUser(r)
	currentToken := ""
	if currentUser != nil {
		currentToken = currentUser.SessionToken()
	}

	// Get all active sessions
	activeSessions, err := h.sessions.GetActiveSessions(ctx, 100)
	if err != nil {
		h.logger.Error("failed to get active sessions", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build user lookup map
	userIDs := make([]primitive.ObjectID, 0, len(activeSessions))
	for _, sess := range activeSessions {
		userIDs = append(userIDs, sess.UserID)
	}
	users, _ := h.userStore.GetByIDs(ctx, userIDs)
	userMap := make(map[primitive.ObjectID]*models.User, len(users))
	for i := range users {
		userMap[users[i].ID] = &users[i]
	}

	// Build view models
	sessionVMs := make([]SessionVM, 0, len(activeSessions))
	now := time.Now()
	for _, sess := range activeSessions {
		vm := SessionVM{
			ID:               sess.ID.Hex(),
			UserID:           sess.UserID.Hex(),
			CurrentPage:      sess.CurrentPage,
			LastActivity:     sess.LastActivity,
			LastActivityAgo:  formatTimeAgo(sess.LastActivity, now),
			IPAddress:        sess.IPAddress,
			DeviceInfo:       parseUserAgent(sess.UserAgent),
			LoginAt:          sess.LoginAt,
			LoginAtFormatted: sess.LoginAt.Format("Jan 2 3:04 PM"),
			IsCurrentSession: sess.Token == currentToken,
		}

		if user, ok := userMap[sess.UserID]; ok {
			vm.UserName = user.FullName
			if user.Email != nil {
				vm.UserEmail = *user.Email
			}
		} else {
			vm.UserName = "Unknown User"
		}

		sessionVMs = append(sessionVMs, vm)
	}

	vm := SessionsListVM{
		BaseVM:       viewdata.New(r),
		Sessions:     sessionVMs,
		CurrentToken: currentToken,
	}
	vm.Title = "Active Sessions"
	vm.BackURL = "/dashboard"

	templates.Render(w, r, "dashboard/sessions", vm)
}

// listSessionsTable returns just the sessions table for HTMX refresh.
func (h *SessionsHandler) listSessionsTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current user's session token
	currentUser, _ := auth.CurrentUser(r)
	currentToken := ""
	if currentUser != nil {
		currentToken = currentUser.SessionToken()
	}

	// Get all active sessions
	activeSessions, err := h.sessions.GetActiveSessions(ctx, 100)
	if err != nil {
		h.logger.Error("failed to get active sessions", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build user lookup map
	userIDs := make([]primitive.ObjectID, 0, len(activeSessions))
	for _, sess := range activeSessions {
		userIDs = append(userIDs, sess.UserID)
	}
	users, _ := h.userStore.GetByIDs(ctx, userIDs)
	userMap := make(map[primitive.ObjectID]*models.User, len(users))
	for i := range users {
		userMap[users[i].ID] = &users[i]
	}

	// Build view models
	sessionVMs := make([]SessionVM, 0, len(activeSessions))
	now := time.Now()
	for _, sess := range activeSessions {
		vm := SessionVM{
			ID:               sess.ID.Hex(),
			UserID:           sess.UserID.Hex(),
			CurrentPage:      sess.CurrentPage,
			LastActivity:     sess.LastActivity,
			LastActivityAgo:  formatTimeAgo(sess.LastActivity, now),
			IPAddress:        sess.IPAddress,
			DeviceInfo:       parseUserAgent(sess.UserAgent),
			LoginAt:          sess.LoginAt,
			LoginAtFormatted: sess.LoginAt.Format("Jan 2 3:04 PM"),
			IsCurrentSession: sess.Token == currentToken,
		}

		if user, ok := userMap[sess.UserID]; ok {
			vm.UserName = user.FullName
			if user.Email != nil {
				vm.UserEmail = *user.Email
			}
		} else {
			vm.UserName = "Unknown User"
		}

		sessionVMs = append(sessionVMs, vm)
	}

	vm := SessionsListVM{
		Sessions:     sessionVMs,
		CurrentToken: currentToken,
	}

	templates.RenderSnippet(w, "dashboard/sessions_table", vm)
}

// terminateSession terminates a session by ID.
func (h *SessionsHandler) terminateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")

	oid, err := primitive.ObjectIDFromHex(sessionID)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// Get the session to check ownership
	sess, err := h.sessions.GetByID(r.Context(), oid)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Don't allow terminating your own session
	currentUser, _ := auth.CurrentUser(r)
	if currentUser != nil && sess.Token == currentUser.SessionToken() {
		http.Error(w, "Cannot terminate your own session", http.StatusBadRequest)
		return
	}

	// Close the session
	if err := h.sessions.Close(r.Context(), sess.Token, sessions.EndReasonInactive); err != nil {
		h.logger.Error("failed to terminate session", zap.Error(err), zap.String("session_id", sessionID))
		http.Error(w, "Failed to terminate session", http.StatusInternalServerError)
		return
	}

	h.logger.Info("session terminated by admin",
		zap.String("session_id", sessionID),
		zap.String("terminated_user_id", sess.UserID.Hex()),
		zap.String("admin_id", currentUser.ID))

	// For HTMX requests, return empty response (row will be removed)
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/dashboard/sessions", http.StatusSeeOther)
}

// formatTimeAgo formats a time as "X ago" string.
func formatTimeAgo(t time.Time, now time.Time) string {
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return formatPlural(mins, "minute") + " ago"
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return formatPlural(hours, "hour") + " ago"
	}
	days := int(diff.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return formatPlural(days, "day") + " ago"
}

func formatPlural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}

// parseUserAgent extracts device info from user agent string.
func parseUserAgent(ua string) string {
	if ua == "" {
		return "Unknown"
	}

	// Simple parsing - could be enhanced with a proper UA parser
	switch {
	case contains(ua, "iPhone"):
		return "iPhone"
	case contains(ua, "iPad"):
		return "iPad"
	case contains(ua, "Android"):
		return "Android"
	case contains(ua, "Mac OS"):
		return "Mac"
	case contains(ua, "Windows"):
		return "Windows"
	case contains(ua, "Linux"):
		return "Linux"
	default:
		return "Browser"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

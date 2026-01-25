// internal/app/features/logout/logout.go
package logout

import (
	"net/http"

	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/auditlog"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Handler provides logout handlers.
type Handler struct {
	sessionMgr    *auth.SessionManager
	auditLogger   *auditlog.Logger
	sessionsStore *sessions.Store
	logger        *zap.Logger
}

// NewHandler creates a new logout Handler.
func NewHandler(
	sessionMgr *auth.SessionManager,
	auditLogger *auditlog.Logger,
	sessionsStore *sessions.Store,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		sessionMgr:    sessionMgr,
		auditLogger:   auditLogger,
		sessionsStore: sessionsStore,
		logger:        logger,
	}
}

// Routes returns a chi.Router with logout routes mounted.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireAuth)
	r.Post("/", h.handleLogout)
	r.Get("/", h.handleLogout) // Allow GET for simple logout links
	return r
}

// handleLogout terminates the session.
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if user, ok := auth.CurrentUser(r); ok {
		h.auditLogger.Logout(r.Context(), r, user.ID)

		// Close session in MongoDB tracking (preserves for audit, records duration)
		// Note: Logout time is captured in the session record (logout_at), so we don't need
		// a separate activity event for logout - it would be redundant.
		if token := user.SessionToken(); token != "" {
			if err := h.sessionsStore.Close(r.Context(), token, sessions.EndReasonLogout); err != nil {
				h.logger.Warn("failed to close session in store", zap.Error(err))
			}
		}
	}

	h.sessionMgr.DestroySession(w, r)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

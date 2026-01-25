// internal/app/features/status/routes.go
package status

import (
	"net/http"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns a chi.Router with status routes mounted.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireRole("admin"))
	r.Get("/", h.Serve)
	r.Post("/renew", h.HandleRenew)
	return r
}

// internal/app/features/jobs/routes.go
package jobsfeature

import (
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns the router for the jobs feature.
// Access is restricted to admin and developer roles.
func Routes(h *Handler, sm *auth.SessionManager) chi.Router {
	r := chi.NewRouter()
	r.Use(sm.RequireRole("admin", "developer"))

	r.Get("/", h.ServeDashboard)
	r.Get("/list", h.ServeList)
	r.Get("/{id}", h.ServeDetail)
	r.Post("/{id}/retry", h.HandleRetry)
	r.Post("/{id}/cancel", h.HandleCancel)

	return r
}

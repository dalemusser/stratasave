// internal/app/features/stats/routes.go
package statsfeature

import (
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns the router for the stats feature.
// Access is restricted to admin and developer roles.
func Routes(h *Handler, sm *auth.SessionManager) chi.Router {
	r := chi.NewRouter()
	r.Use(sm.RequireRole("admin", "developer"))

	r.Get("/", h.ServeDashboard)
	r.Get("/detail", h.ServeDetail)

	return r
}

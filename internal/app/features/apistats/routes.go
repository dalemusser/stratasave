package apistats

import (
	"net/http"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns the router for API stats feature.
// Admin and developer roles can view stats.
// Only admin can change recording settings and manage data.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()

	// Require admin or developer role for viewing
	r.Use(sessionMgr.RequireRole("admin", "developer"))

	// Main page - viewable by admin and developer
	r.Get("/", h.ServeList)

	// Chart data API - viewable by admin and developer
	r.Get("/chart-data", h.ServeChartData)

	// Admin-only operations
	r.Group(func(r chi.Router) {
		r.Use(sessionMgr.RequireRole("admin"))

		// Update bucket duration
		r.Post("/bucket", h.HandleSetBucket)

		// Roll-up operations
		r.Post("/rollup", h.HandleRollUp)

		// Delete operations
		r.Post("/delete", h.HandleDelete)
	})

	return r
}

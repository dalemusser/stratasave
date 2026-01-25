// internal/app/features/activity/routes.go
package activity

import (
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns the router for activity dashboard endpoints.
// Admin only - can view all user activity.
func Routes(h *Handler, sm *auth.SessionManager) chi.Router {
	r := chi.NewRouter()

	// All activity routes require admin role
	r.Group(func(pr chi.Router) {
		pr.Use(sm.RequireSignedIn)
		pr.Use(sm.RequireRole("admin"))

		// Real-time dashboard ("Who's Online")
		pr.Get("/", h.ServeDashboard)

		// HTMX partial for refreshing the online status table
		pr.Get("/online-table", h.ServeOnlineTable)

		// Weekly summary view
		pr.Get("/summary", h.ServeSummary)

		// User detail view
		pr.Get("/user/{userID}", h.ServeUserDetail)

		// HTMX partial for refreshing user detail content
		pr.Get("/user/{userID}/content", h.ServeUserDetailContent)

		// Export UI
		pr.Get("/export", h.ServeExport)

		// CSV/JSON exports
		pr.Get("/export/sessions.csv", h.ServeSessionsCSV)
		pr.Get("/export/sessions.json", h.ServeSessionsJSON)
		pr.Get("/export/events.csv", h.ServeEventsCSV)
		pr.Get("/export/events.json", h.ServeEventsJSON)
	})

	return r
}

package savebrowser

import (
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns the router for the save browser feature.
// Access is restricted to admin and developer roles.
func Routes(h *Handler, sm *auth.SessionManager) chi.Router {
	r := chi.NewRouter()
	r.Use(sm.RequireRole("admin", "developer"))

	// Main browser page
	r.Get("/", h.ServeList)

	// HTMX partials
	r.Get("/game-picker", h.ServeGamePicker)
	r.Get("/players", h.ServePlayers)
	r.Get("/data", h.ServeSaves)

	// Playground - interactive API testing
	r.Get("/playground", h.ServePlayground)
	r.Post("/playground/execute", h.HandlePlaygroundExecute)

	// Documentation
	r.Get("/docs", h.ServeDocs)

	// Create (for dev tool)
	r.Post("/create", h.HandleCreateState)

	// Delete operations
	r.Post("/{game}/{id}/delete", h.HandleDeleteSave)
	r.Post("/{game}/user/{userID}/delete", h.HandleDeleteUserSaves)

	return r
}

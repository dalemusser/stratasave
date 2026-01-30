package settingsbrowser

import (
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns the router for the settings browser feature.
// Access is restricted to admin and developer roles.
func Routes(h *Handler, sm *auth.SessionManager) chi.Router {
	r := chi.NewRouter()
	r.Use(sm.RequireRole("admin", "developer"))

	// Main browser page
	r.Get("/", h.ServeList)

	// HTMX partials
	r.Get("/game-picker", h.ServeGamePicker)
	r.Get("/users", h.ServeUsers)
	r.Get("/data", h.ServeSetting)

	// Playground - interactive API testing
	r.Get("/playground", h.ServePlayground)
	r.Post("/playground/execute", h.HandlePlaygroundExecute)

	// Documentation
	r.Get("/docs", h.ServeDocs)

	// Create (for dev tool)
	r.Post("/create", h.HandleCreateSetting)

	// Delete operations
	r.Post("/{game}/user/{userID}/delete", h.HandleDeleteSetting)

	return r
}

// internal/app/features/apikeys/routes.go
package apikeysfeature

import (
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns the router for the API keys feature.
// Access is restricted to admin role only.
func Routes(h *Handler, sm *auth.SessionManager) chi.Router {
	r := chi.NewRouter()
	r.Use(sm.RequireRole("admin"))

	r.Get("/", h.ServeList)
	r.Get("/new", h.ServeNew)
	r.Post("/", h.HandleCreate)
	r.Get("/{id}", h.ServeDetail)
	r.Get("/{id}/edit", h.ServeEdit)
	r.Get("/{id}/manage_modal", h.ServeManageModal)
	r.Post("/{id}/edit", h.HandleUpdate)
	r.Post("/{id}/revoke", h.HandleRevoke)
	r.Post("/{id}/delete", h.HandleDelete)

	return r
}

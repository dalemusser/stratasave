// internal/app/features/ledger/routes.go
package ledgerfeature

import (
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
)

// Routes returns the router for ledger feature.
// Access is restricted to admin and developer roles.
func Routes(h *Handler, sm *auth.SessionManager) chi.Router {
	r := chi.NewRouter()
	r.Use(sm.RequireRole("admin", "developer"))

	r.Get("/", h.ServeList)
	r.Get("/stats", h.ServeStats)
	r.Get("/{id}", h.ServeDetail)
	r.Post("/{id}/delete", h.HandleDelete)
	r.Post("/delete-range", h.HandleDeleteRange)

	return r
}

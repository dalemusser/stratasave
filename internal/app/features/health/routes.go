// internal/app/features/health/routes.go
package health

import (
	"github.com/dalemusser/stratasave/internal/app/system/handler"
	"github.com/go-chi/chi/v5"
)

func MountRoutes(r chi.Router, h *handler.Handler) {
	r.Get("/health", (&Handler{h}).Serve)
}

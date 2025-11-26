// internal/app/system/routes/routes.go
package routes

import (
	"github.com/dalemusser/stratasave/internal/app/features/health"
	"github.com/dalemusser/stratasave/internal/app/system/handler"
	"github.com/go-chi/chi/v5"
)

// RegisterAllRoutes mounts the routes for every feature in one place,
// passing `h` to each feature that needs DB or config references.
func RegisterAllRoutes(r chi.Router, h *handler.Handler) {
	health.MountRoutes(r, h)
}

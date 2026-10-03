package settingsapi

import (
	"net/http"

	apistatsstore "github.com/dalemusser/stratasave/internal/app/store/apistats"
	"github.com/dalemusser/stratasave/internal/app/system/apicors"
	"github.com/dalemusser/stratasave/internal/app/system/apistats"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Routes returns a router with the settings API endpoints.
//
// When mounted at /api/settings:
//   - POST /api/settings/save - Save player settings
//   - POST /api/settings/load - Load player settings
//   - POST /api/settings/delete - Delete a user's saved settings
//
// Authentication is via API key (Bearer token in Authorization header).
// Save and load accept apiKeys, the keys game clients hold; delete accepts
// deleteKeys, held by servers only (see saveapi.Routes).
// CORS is permissive (allows any origin) since API key auth is used.
func Routes(h *Handler, recorder *apistats.Recorder, apiKeys, deleteKeys []string, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// API CORS - permissive for API key auth
	r.Use(apicors.Middleware())

	// Save endpoint with stats tracking
	r.Route("/save", func(sr chi.Router) {
		sr.Use(auth.APIKeyAuth(apiKeys, logger))
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeSaveSettings))
		sr.Post("/", h.SaveHandler)
	})

	// Load endpoint with stats tracking
	r.Route("/load", func(sr chi.Router) {
		sr.Use(auth.APIKeyAuth(apiKeys, logger))
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeLoadSettings))
		sr.Post("/", h.LoadHandler)
	})

	// Delete endpoint with stats tracking
	r.Route("/delete", func(sr chi.Router) {
		sr.Use(auth.APIKeyAuth(deleteKeys, logger))
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeDeleteSettings))
		sr.Post("/", h.DeleteHandler)
	})

	return r
}

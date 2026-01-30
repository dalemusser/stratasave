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
//
// Authentication is via API key (Bearer token in Authorization header).
// CORS is permissive (allows any origin) since API key auth is used.
func Routes(h *Handler, recorder *apistats.Recorder, apiKey string, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// API CORS - permissive for API key auth
	r.Use(apicors.Middleware())

	// API key authentication
	r.Use(auth.APIKeyAuth(apiKey, logger))

	// Save endpoint with stats tracking
	r.Route("/save", func(sr chi.Router) {
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeSaveSettings))
		sr.Post("/", h.SaveHandler)
	})

	// Load endpoint with stats tracking
	r.Route("/load", func(sr chi.Router) {
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeLoadSettings))
		sr.Post("/", h.LoadHandler)
	})

	return r
}

package saveapi

import (
	"net/http"

	apistatsstore "github.com/dalemusser/stratasave/internal/app/store/apistats"
	"github.com/dalemusser/stratasave/internal/app/system/apicors"
	"github.com/dalemusser/stratasave/internal/app/system/apistats"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Routes returns a router with the state save/load API endpoints.
//
// When mounted at /api/state:
//   - POST /api/state/save - Save game state
//   - POST /api/state/load - Load game state
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
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeSaveState))
		sr.Post("/", h.SaveHandler)
	})

	// Load endpoint with stats tracking
	r.Route("/load", func(sr chi.Router) {
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeLoadState))
		sr.Post("/", h.LoadHandler)
	})

	return r
}

// LegacyRoutes returns a router with legacy endpoints at root level.
//
// These are for backward compatibility:
//   - POST /save - Save game state (legacy)
//   - POST /load - Load game state (legacy)
//
// New integrations should use /api/state/save and /api/state/load instead.
func LegacyRoutes(h *Handler, recorder *apistats.Recorder, apiKey string, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// API CORS - permissive for API key auth
	r.Use(apicors.Middleware())

	// API key authentication
	r.Use(auth.APIKeyAuth(apiKey, logger))

	// Legacy save endpoint
	r.Group(func(sr chi.Router) {
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeSaveState))
		sr.Post("/", h.SaveHandler)
	})

	return r
}

// LegacyLoadRoutes returns a router for the legacy /load endpoint.
func LegacyLoadRoutes(h *Handler, recorder *apistats.Recorder, apiKey string, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// API CORS - permissive for API key auth
	r.Use(apicors.Middleware())

	// API key authentication
	r.Use(auth.APIKeyAuth(apiKey, logger))

	// Legacy load endpoint
	r.Group(func(sr chi.Router) {
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeLoadState))
		sr.Post("/", h.LoadHandler)
	})

	return r
}

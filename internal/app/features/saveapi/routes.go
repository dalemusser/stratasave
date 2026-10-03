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

// Routes returns a router with the state save/load/delete API endpoints.
//
// When mounted at /api/state:
//   - POST /api/state/save - Save game state
//   - POST /api/state/load - Load game state
//   - POST /api/state/delete - Delete a user's saved game state
//
// Authentication is via API key (Bearer token in Authorization header).
// Save and load accept apiKeys, the keys game clients hold, and the
// restricted key for its own user ids. Delete accepts deleteKeys: a game
// client's key is visible in the page that hosts the game, so it must not
// be able to remove a student's saves; the delete key is held by servers
// only.
// CORS is permissive (allows any origin) since API key auth is used.
func Routes(h *Handler, recorder *apistats.Recorder, apiKeys []string, restricted auth.RestrictedKey, deleteKeys []string, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// API CORS - permissive for API key auth
	r.Use(apicors.Middleware())

	// Save endpoint with stats tracking
	r.Route("/save", func(sr chi.Router) {
		sr.Use(auth.APIKeyAuthRestricted(apiKeys, restricted, logger))
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeSaveState))
		sr.Post("/", h.SaveHandler)
	})

	// Load endpoint with stats tracking
	r.Route("/load", func(sr chi.Router) {
		sr.Use(auth.APIKeyAuthRestricted(apiKeys, restricted, logger))
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeLoadState))
		sr.Post("/", h.LoadHandler)
	})

	// Delete endpoint with stats tracking
	r.Route("/delete", func(sr chi.Router) {
		sr.Use(auth.APIKeyAuth(deleteKeys, logger))
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeDeleteState))
		sr.Post("/", h.DeleteHandler)
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
func LegacyRoutes(h *Handler, recorder *apistats.Recorder, apiKeys []string, restricted auth.RestrictedKey, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// API CORS - permissive for API key auth
	r.Use(apicors.Middleware())

	// API key authentication
	r.Use(auth.APIKeyAuthRestricted(apiKeys, restricted, logger))

	// Legacy save endpoint
	r.Group(func(sr chi.Router) {
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeSaveState))
		sr.Post("/", h.SaveHandler)
	})

	return r
}

// LegacyLoadRoutes returns a router for the legacy /load endpoint.
func LegacyLoadRoutes(h *Handler, recorder *apistats.Recorder, apiKeys []string, restricted auth.RestrictedKey, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// API CORS - permissive for API key auth
	r.Use(apicors.Middleware())

	// API key authentication
	r.Use(auth.APIKeyAuthRestricted(apiKeys, restricted, logger))

	// Legacy load endpoint
	r.Group(func(sr chi.Router) {
		sr.Use(apistats.MiddlewareWithRecorder(recorder, apistatsstore.StatTypeLoadState))
		sr.Post("/", h.LoadHandler)
	})

	return r
}

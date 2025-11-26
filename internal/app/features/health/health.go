// internal/app/features/health/health.go
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/handler"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

// Handler keeps a reference to the shared application handler.
type Handler struct{ h *handler.Handler }

// Serve handles GET /health
//
// Response (200):
//
//	{ "status":"ok", "database":"connected" }
//
// Response (503):
//
//	{ "status":"error", "message":"Database unavailable", "error":"…"}
func (hh *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")

	if err := hh.h.Client.Ping(ctx, readpref.Primary()); err != nil {
		zap.L().Error("health-check: mongo ping failed", zap.Error(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Database unavailable",
			"error":   err.Error(),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"database": "connected",
	})
}

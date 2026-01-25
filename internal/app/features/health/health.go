// internal/app/features/health/health.go
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

// Handler provides health check endpoints.
type Handler struct {
	mongoClient *mongo.Client
	logger      *zap.Logger
}

// NewHandler creates a new health check Handler.
func NewHandler(mongoClient *mongo.Client, logger *zap.Logger) *Handler {
	return &Handler{
		mongoClient: mongoClient,
		logger:      logger,
	}
}

// Response represents the health check response.
type Response struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services,omitempty"`
}

// Routes returns a chi.Router with health check routes mounted.
// Provides /health (full check), /health/ready, and /health/live.
func Routes(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.Check)
	r.Get("/ready", h.Ready)
	r.Get("/live", h.Live)
	return r
}

// MountRootEndpoints adds /ready and /livez endpoints directly on the root router.
// This is the standard convention for Kubernetes probes:
//   - /ready (or /readyz) - readiness probe
//   - /livez - liveness probe
func MountRootEndpoints(r chi.Router, h *Handler) {
	r.Get("/ready", h.Ready)
	r.Get("/readyz", h.Ready)
	r.Get("/livez", h.Live)
}

// Check performs a full health check including database connectivity.
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	resp := Response{
		Status:   "ok",
		Services: make(map[string]string),
	}

	// Check MongoDB
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		resp.Status = "degraded"
		resp.Services["mongodb"] = "unavailable"
		h.logger.Warn("health check: mongodb ping failed", zap.Error(err))
	} else {
		resp.Services["mongodb"] = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	if resp.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(resp)
}

// Ready checks if the service is ready to accept requests.
// Used by Kubernetes readiness probes.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		h.logger.Warn("readiness check failed", zap.Error(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not ready"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ready"}`))
}

// Live checks if the service is alive.
// Used by Kubernetes liveness probes.
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"alive"}`))
}

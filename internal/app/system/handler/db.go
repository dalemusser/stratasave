// internal/app/system/handler/db.go
package handler

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

// Standard DB timeout presets used across features.
// Use these with DBWithTimeout, e.g.:
//
//	ctx, cancel, db := h.DBWithTimeout(r, handler.DBTimeoutShort)
//
// or the convenience helpers below:
//
//	ctx, cancel, db := h.DBShort(r)
const (
	// Short reads / list pages / lightweight aggregations
	DBTimeoutShort = 5 * time.Second

	// Single inserts/updates/deletes (non-batch)
	DBTimeoutMed = 6 * time.Second

	// Heavier reads / longer-running aggregates / CSV uploads
	DBTimeoutLong = 10 * time.Second
)

func (h *Handler) DB() *mongo.Database {
	return h.Client.Database(h.Cfg.MongoDatabase)
}

func (h *Handler) WithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

// DBWithTimeout returns a context with timeout and the configured DB.
// Callers must defer cancel().
func (h *Handler) DBWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc, *mongo.Database) {
	ctx, cancel := context.WithTimeout(r.Context(), d)
	return ctx, cancel, h.Client.Database(h.Cfg.MongoDatabase)
}

// Convenience wrappers using the standard presets.
// These are optional sugar—use DBWithTimeout if you prefer explicit durations.
func (h *Handler) DBShort(r *http.Request) (context.Context, context.CancelFunc, *mongo.Database) {
	return h.DBWithTimeout(r, DBTimeoutShort)
}
func (h *Handler) DBMed(r *http.Request) (context.Context, context.CancelFunc, *mongo.Database) {
	return h.DBWithTimeout(r, DBTimeoutMed)
}
func (h *Handler) DBLong(r *http.Request) (context.Context, context.CancelFunc, *mongo.Database) {
	return h.DBWithTimeout(r, DBTimeoutLong)
}

// Package apistats provides middleware for tracking API request statistics.
package apistats

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/apistats"
	"go.uber.org/zap"
)

// Config holds configuration for the API stats middleware.
type Config struct {
	// Store is the API stats store for persisting statistics.
	Store *apistats.Store

	// Logger for logging errors.
	Logger *zap.Logger

	// BucketDuration is the time bucket size for aggregation.
	// Common values: 1*time.Minute, 15*time.Minute, 1*time.Hour, 24*time.Hour
	BucketDuration time.Duration

	// StatType identifies the type of API operation.
	StatType apistats.StatType
}

// Recorder provides methods to record API statistics.
// It can be shared across handlers and supports dynamic bucket duration changes.
type Recorder struct {
	store          *apistats.Store
	logger         *zap.Logger
	bucketDuration time.Duration
	mu             sync.RWMutex
}

// NewRecorder creates a new API stats recorder.
func NewRecorder(store *apistats.Store, logger *zap.Logger, defaultBucketDuration time.Duration) *Recorder {
	return &Recorder{
		store:          store,
		logger:         logger,
		bucketDuration: defaultBucketDuration,
	}
}

// SetBucketDuration updates the bucket duration for new recordings.
// This is safe to call concurrently.
func (r *Recorder) SetBucketDuration(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bucketDuration = d
}

// GetBucketDuration returns the current bucket duration.
func (r *Recorder) GetBucketDuration() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bucketDuration
}

// Record records a single API request's statistics asynchronously.
func (r *Recorder) Record(statType apistats.StatType, durationMs int64, isError bool) {
	r.mu.RLock()
	bucketDuration := r.bucketDuration
	r.mu.RUnlock()

	// Record asynchronously to not block the response
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := r.store.Record(ctx, statType, bucketDuration, durationMs, isError); err != nil {
			r.logger.Error("failed to record API stats",
				zap.String("stat_type", string(statType)),
				zap.Error(err),
			)
		}
	}()
}

// Middleware returns HTTP middleware that records API statistics.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call the next handler
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)
			durationMs := duration.Milliseconds()

			// Determine if this was an error
			isError := wrapped.statusCode >= 400

			// Record asynchronously
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := cfg.Store.Record(ctx, cfg.StatType, cfg.BucketDuration, durationMs, isError); err != nil {
					cfg.Logger.Error("failed to record API stats",
						zap.String("stat_type", string(cfg.StatType)),
						zap.Int64("duration_ms", durationMs),
						zap.Error(err),
					)
				}
			}()
		})
	}
}

// MiddlewareWithRecorder returns HTTP middleware using a shared recorder.
// This allows dynamic bucket duration changes.
// If recorder is nil, stats recording is skipped (useful for testing).
func MiddlewareWithRecorder(recorder *Recorder, statType apistats.StatType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// If no recorder, just pass through
		if recorder == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call the next handler
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)
			durationMs := duration.Milliseconds()

			// Determine if this was an error
			isError := wrapped.statusCode >= 400

			// Record using the shared recorder
			recorder.Record(statType, durationMs, isError)
		})
	}
}

// responseWrapper wraps http.ResponseWriter to capture status code.
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWrapper) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher.
func (rw *responseWrapper) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

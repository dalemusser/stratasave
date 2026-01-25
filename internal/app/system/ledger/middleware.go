// internal/app/system/ledger/middleware.go
package ledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/ledger"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ctxKey is the context key type for ledger data.
type ctxKey int

const (
	ctxKeyEntry ctxKey = iota
	ctxKeyTiming
)

// Config holds configuration for the ledger middleware.
type Config struct {
	// Store is the ledger store for persisting entries.
	Store *ledgerstore.Store

	// Logger for logging errors.
	Logger *zap.Logger

	// MaxBodyPreview is the maximum number of characters to capture from request body.
	// Set to 0 to disable body preview capture.
	MaxBodyPreview int

	// HeadersToCapture is a list of header names to capture.
	// Sensitive headers like Authorization are automatically redacted.
	HeadersToCapture []string

	// ExcludePaths is a list of path prefixes to exclude from logging.
	// Common examples: "/health", "/static", "/assets"
	ExcludePaths []string

	// OnlyAPIPaths restricts logging to paths starting with these prefixes.
	// If empty, all paths are logged (except ExcludePaths).
	OnlyAPIPaths []string

	// CaptureErrors determines whether to capture error details.
	CaptureErrors bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(store *ledgerstore.Store, logger *zap.Logger) Config {
	return Config{
		Store:          store,
		Logger:         logger,
		MaxBodyPreview: 500,
		HeadersToCapture: []string{
			"Content-Type",
			"Accept",
			"User-Agent",
			"X-Request-ID",
			"X-Forwarded-For",
		},
		ExcludePaths: []string{
			"/health",
			"/static",
			"/assets",
			"/favicon.ico",
		},
		CaptureErrors: true,
	}
}

// Middleware returns HTTP middleware that logs requests to the ledger.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should be excluded
			path := r.URL.Path
			for _, prefix := range cfg.ExcludePaths {
				if strings.HasPrefix(path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check if path should be included (if OnlyAPIPaths is set)
			if len(cfg.OnlyAPIPaths) > 0 {
				included := false
				for _, prefix := range cfg.OnlyAPIPaths {
					if strings.HasPrefix(path, prefix) {
						included = true
						break
					}
				}
				if !included {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Generate request ID
			requestID := uuid.New().String()

			// Check for client-provided request ID
			clientRequestID := r.Header.Get("X-Request-ID")

			// Extract trace ID if present
			traceID := r.Header.Get("X-Trace-ID")

			// Start timing
			startTime := time.Now()
			timing := &TimingContext{
				phases: make(map[string]float64),
			}

			// Capture request body if needed
			var bodyPreview string
			var bodyHash string
			var bodySize int64
			if cfg.MaxBodyPreview > 0 && r.Body != nil && r.ContentLength > 0 {
				body, err := io.ReadAll(r.Body)
				if err == nil {
					bodySize = int64(len(body))
					if len(body) > 0 {
						// Compute hash
						hash := sha256.Sum256(body)
						bodyHash = hex.EncodeToString(hash[:])[:8]

						// Capture preview (truncate if needed)
						preview := string(body)
						if len(preview) > cfg.MaxBodyPreview {
							preview = preview[:cfg.MaxBodyPreview] + "..."
						}
						bodyPreview = preview
					}
					// Restore body for handler
					r.Body = io.NopCloser(bytes.NewReader(body))
				}
			}

			// Capture headers
			headers := make(map[string]string)
			for _, name := range cfg.HeadersToCapture {
				if value := r.Header.Get(name); value != "" {
					// Redact sensitive values
					if strings.EqualFold(name, "Authorization") {
						if len(value) > 10 {
							headers[name] = value[:10] + "..."
						} else {
							headers[name] = "[redacted]"
						}
					} else {
						headers[name] = value
					}
				}
			}

			// Determine actor type and ID
			actorType := "anonymous"
			actorID := ""
			actorName := ""

			// Check for session user
			if user, ok := auth.CurrentUser(r); ok {
				actorType = "session"
				actorID = user.ID
				actorName = user.Name
			}

			// Check for API key (set by API key middleware)
			if apiKeyID := r.Header.Get("X-API-Key-ID"); apiKeyID != "" {
				actorType = "api_key"
				actorID = apiKeyID
				actorName = r.Header.Get("X-API-Key-Name")
			}

			// Create initial entry
			entry := &ledgerstore.Entry{
				RequestID:          requestID,
				TraceID:            traceID,
				ClientRequestID:    clientRequestID,
				Method:             r.Method,
				Path:               path,
				Query:              r.URL.RawQuery,
				Headers:            headers,
				RemoteIP:           extractIP(r),
				ActorType:          actorType,
				ActorID:            actorID,
				ActorName:          actorName,
				RequestBodySize:    bodySize,
				RequestBodyHash:    bodyHash,
				RequestBodyPreview: bodyPreview,
				RequestContentType: r.Header.Get("Content-Type"),
				StartedAt:          startTime,
				Metadata:           make(map[string]any),
			}

			// Add entry and timing to context
			ctx := context.WithValue(r.Context(), ctxKeyEntry, entry)
			ctx = context.WithValue(ctx, ctxKeyTiming, timing)
			r = r.WithContext(ctx)

			// Wrap response writer to capture status code and size
			wrapped := &responseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call next handler
			next.ServeHTTP(wrapped, r)

			// Complete timing
			endTime := time.Now()
			timing.TotalMs = float64(endTime.Sub(startTime).Microseconds()) / 1000.0

			// Complete entry
			entry.StatusCode = wrapped.statusCode
			entry.ResponseSize = wrapped.bytesWritten
			entry.CompletedAt = endTime
			entry.Timing = ledgerstore.TimingInfo{
				DecodeMs:   timing.phases["decode"],
				ValidateMs: timing.phases["validate"],
				DBQueryMs:  timing.phases["db"],
				EncodeMs:   timing.phases["encode"],
				TotalMs:    timing.TotalMs,
			}

			// Capture error info if present
			if cfg.CaptureErrors && wrapped.statusCode >= 400 {
				if errClass := GetErrorClass(ctx); errClass != "" {
					entry.ErrorClass = errClass
				} else {
					// Classify by status code
					switch {
					case wrapped.statusCode == 400:
						entry.ErrorClass = "validation"
					case wrapped.statusCode == 401:
						entry.ErrorClass = "auth"
					case wrapped.statusCode == 403:
						entry.ErrorClass = "forbidden"
					case wrapped.statusCode == 404:
						entry.ErrorClass = "not_found"
					case wrapped.statusCode >= 500:
						entry.ErrorClass = "internal"
					default:
						entry.ErrorClass = "client_error"
					}
				}
				if errMsg := GetErrorMessage(ctx); errMsg != "" {
					entry.ErrorMessage = errMsg
				}
			}

			// Store entry asynchronously to not block response
			go func() {
				storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := cfg.Store.Create(storeCtx, *entry); err != nil {
					cfg.Logger.Error("failed to store ledger entry",
						zap.String("request_id", requestID),
						zap.Error(err))
				}
			}()
		})
	}
}

// responseWrapper wraps http.ResponseWriter to capture status code and bytes written.
type responseWrapper struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWrapper) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// Flush implements http.Flusher.
func (rw *responseWrapper) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// extractIP extracts the client IP from the request.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// TimingContext holds timing information for a request.
type TimingContext struct {
	phases  map[string]float64
	current string
	start   time.Time
	TotalMs float64
}

// StartTiming starts timing a phase.
func StartTiming(ctx context.Context, phase string) context.Context {
	timing, ok := ctx.Value(ctxKeyTiming).(*TimingContext)
	if !ok {
		return ctx
	}
	// End any current phase
	if timing.current != "" {
		elapsed := float64(time.Since(timing.start).Microseconds()) / 1000.0
		timing.phases[timing.current] += elapsed
	}
	timing.current = phase
	timing.start = time.Now()
	return ctx
}

// EndTiming ends timing the current phase.
func EndTiming(ctx context.Context) {
	timing, ok := ctx.Value(ctxKeyTiming).(*TimingContext)
	if !ok || timing.current == "" {
		return
	}
	elapsed := float64(time.Since(timing.start).Microseconds()) / 1000.0
	timing.phases[timing.current] += elapsed
	timing.current = ""
}

// AddMetadata adds metadata to the ledger entry.
func AddMetadata(ctx context.Context, key string, value any) {
	entry, ok := ctx.Value(ctxKeyEntry).(*ledgerstore.Entry)
	if !ok {
		return
	}
	if entry.Metadata == nil {
		entry.Metadata = make(map[string]any)
	}
	entry.Metadata[key] = value
}

// SetErrorClass sets the error class for the ledger entry.
func SetErrorClass(ctx context.Context, class string) {
	entry, ok := ctx.Value(ctxKeyEntry).(*ledgerstore.Entry)
	if !ok {
		return
	}
	entry.ErrorClass = class
}

// SetErrorMessage sets the error message for the ledger entry.
func SetErrorMessage(ctx context.Context, message string) {
	entry, ok := ctx.Value(ctxKeyEntry).(*ledgerstore.Entry)
	if !ok {
		return
	}
	entry.ErrorMessage = message
}

// GetErrorClass returns the error class from context.
func GetErrorClass(ctx context.Context) string {
	entry, ok := ctx.Value(ctxKeyEntry).(*ledgerstore.Entry)
	if !ok {
		return ""
	}
	return entry.ErrorClass
}

// GetErrorMessage returns the error message from context.
func GetErrorMessage(ctx context.Context) string {
	entry, ok := ctx.Value(ctxKeyEntry).(*ledgerstore.Entry)
	if !ok {
		return ""
	}
	return entry.ErrorMessage
}

// GetRequestID returns the request ID for the current request.
func GetRequestID(ctx context.Context) string {
	entry, ok := ctx.Value(ctxKeyEntry).(*ledgerstore.Entry)
	if !ok {
		return ""
	}
	return entry.RequestID
}

// internal/app/features/errors/errors.go
package errors

import (
	"net/http"

	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"go.uber.org/zap"
)

// ErrorLogger wraps the zap logger for error logging.
type ErrorLogger struct {
	logger *zap.Logger
}

// NewErrorLogger creates a new ErrorLogger.
func NewErrorLogger(logger *zap.Logger) *ErrorLogger {
	return &ErrorLogger{logger: logger}
}

// Log logs an error with the given message and error.
func (e *ErrorLogger) Log(r *http.Request, msg string, err error) {
	e.logger.Error(msg,
		zap.Error(err),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
	)
}

// LogWithFields logs an error with additional fields.
func (e *ErrorLogger) LogWithFields(r *http.Request, msg string, err error, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.Error(err),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
	}, fields...)
	e.logger.Error(msg, allFields...)
}

// Handler provides error page handlers.
type Handler struct{}

// NewHandler creates a new error Handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Forbidden renders the 403 forbidden page.
func (h *Handler) Forbidden(w http.ResponseWriter, r *http.Request) {
	vm := viewdata.New(r)
	vm.Title = "Access Denied"

	w.WriteHeader(http.StatusForbidden)
	templates.Render(w, r, "errors/forbidden", vm)
}

// Unauthorized renders the 401 unauthorized page.
func (h *Handler) Unauthorized(w http.ResponseWriter, r *http.Request) {
	vm := viewdata.New(r)
	vm.Title = "Unauthorized"

	w.WriteHeader(http.StatusUnauthorized)
	templates.Render(w, r, "errors/unauthorized", vm)
}

// NotFound renders the 404 not found page.
func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	vm := viewdata.New(r)
	vm.Title = "Not Found"

	w.WriteHeader(http.StatusNotFound)
	templates.Render(w, r, "errors/not_found", vm)
}

// InternalError renders the 500 internal server error page.
func (h *Handler) InternalError(w http.ResponseWriter, r *http.Request) {
	vm := viewdata.New(r)
	vm.Title = "Server Error"

	w.WriteHeader(http.StatusInternalServerError)
	templates.Render(w, r, "errors/internal", vm)
}

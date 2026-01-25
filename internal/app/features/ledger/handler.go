// internal/app/features/ledger/handler.go
package ledgerfeature

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	ledgerstore "github.com/dalemusser/stratasave/internal/app/store/ledger"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler handles ledger-related HTTP requests.
type Handler struct {
	DB     *mongo.Database
	ErrLog *errorsfeature.ErrorLogger
	Log    *zap.Logger
}

// NewHandler creates a new ledger handler.
func NewHandler(db *mongo.Database, errLog *errorsfeature.ErrorLogger, logger *zap.Logger) *Handler {
	return &Handler{
		DB:     db,
		ErrLog: errLog,
		Log:    logger,
	}
}

// ServeList handles GET /ledger - list ledger entries with filtering.
func (h *Handler) ServeList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	// Parse query params
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	filter := ledgerstore.ListFilter{
		ActorType: r.URL.Query().Get("actor_type"),
		ActorID:   r.URL.Query().Get("actor_id"),
		Method:    r.URL.Query().Get("method"),
		Path:      r.URL.Query().Get("path"),
	}

	// Parse time range
	if start := r.URL.Query().Get("start_time"); start != "" {
		if t, err := time.Parse("2006-01-02", start); err == nil {
			filter.StartTime = &t
		}
	}
	if end := r.URL.Query().Get("end_time"); end != "" {
		if t, err := time.Parse("2006-01-02", end); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Second)
			filter.EndTime = &endOfDay
		}
	}

	// Parse status code range
	if min := r.URL.Query().Get("status_min"); min != "" {
		if v, err := strconv.Atoi(min); err == nil {
			filter.StatusCodeMin = &v
		}
	}
	if max := r.URL.Query().Get("status_max"); max != "" {
		if v, err := strconv.Atoi(max); err == nil {
			filter.StatusCodeMax = &v
		}
	}

	filter.ErrorClass = r.URL.Query().Get("error_class")
	filter.Search = r.URL.Query().Get("search")

	store := ledgerstore.New(h.DB)
	result, err := store.List(ctx, filter, page, 50)
	if err != nil {
		h.ErrLog.Log(r, "failed to load ledger entries", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Convert to view models
	entries := make([]LedgerEntryVM, len(result.Entries))
	for i, e := range result.Entries {
		entries[i] = toLedgerEntryVM(e)
	}

	// Compute pagination values
	prevPage := result.Page - 1
	if prevPage < 1 {
		prevPage = 1
	}
	nextPage := result.Page + 1
	if nextPage > result.TotalPages {
		nextPage = result.TotalPages
	}

	base := viewdata.NewBaseVM(r, h.DB, "Request Ledger", "/dashboard")
	data := LedgerListVM{
		BaseVM:     base,
		Entries:    entries,
		Filter:     filter,
		Page:       result.Page,
		TotalPages: result.TotalPages,
		TotalCount: result.TotalCount,
		PrevPage:   prevPage,
		NextPage:   nextPage,
	}

	// Handle HTMX partial render
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "ledger-table" {
		templates.RenderSnippet(w, "ledger_table", data)
		return
	}

	templates.Render(w, r, "ledger/list", data)
}

// ServeDetail handles GET /ledger/{id} - view a single ledger entry.
func (h *Handler) ServeDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := ledgerstore.New(h.DB)
	entry, err := store.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to load ledger entry", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	base := viewdata.NewBaseVM(r, h.DB, "Request Details", "/ledger")
	data := LedgerDetailVM{
		BaseVM: base,
		Entry:  toLedgerEntryVM(*entry),
	}

	templates.Render(w, r, "ledger/detail", data)
}

// ServeStats handles GET /ledger/stats - view ledger statistics.
func (h *Handler) ServeStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	// Default to last 24 hours
	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			start = t
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			end = t.Add(24*time.Hour - time.Second)
		}
	}

	store := ledgerstore.New(h.DB)

	// Get status counts
	statusCounts, err := store.CountByStatus(ctx, start, end)
	if err != nil {
		h.ErrLog.Log(r, "failed to load status counts", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get average response time
	avgResponseTime, err := store.AverageResponseTime(ctx, start, end)
	if err != nil {
		h.ErrLog.Log(r, "failed to load response time", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get recent errors
	recentErrors, err := store.RecentErrors(ctx, 10)
	if err != nil {
		h.ErrLog.Log(r, "failed to load recent errors", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	errorVMs := make([]LedgerEntryVM, len(recentErrors))
	for i, e := range recentErrors {
		errorVMs[i] = toLedgerEntryVM(e)
	}

	// Calculate totals
	var total int64
	for _, count := range statusCounts {
		total += count
	}

	// Calculate total errors (4xx + 5xx)
	totalErrors := statusCounts["4xx"] + statusCounts["5xx"]

	// Build status breakdown with pre-computed percentages
	statusOrder := []string{"2xx", "3xx", "4xx", "5xx"}
	statusBreakdown := make([]StatusBreakdownVM, 0, len(statusOrder))
	for _, status := range statusOrder {
		count := statusCounts[status]
		pct := 0
		if total > 0 {
			pct = int((count * 100) / total)
		}
		statusBreakdown = append(statusBreakdown, StatusBreakdownVM{
			Status:     status,
			Count:      count,
			Percentage: pct,
		})
	}

	base := viewdata.NewBaseVM(r, h.DB, "Ledger Statistics", "/ledger")
	data := LedgerStatsVM{
		BaseVM:          base,
		StartDate:       start.Format("2006-01-02"),
		EndDate:         end.Format("2006-01-02"),
		TotalRequests:   total,
		StatusCounts:    statusCounts,
		StatusBreakdown: statusBreakdown,
		TotalErrors:     totalErrors,
		AvgResponseTime: avgResponseTime,
		RecentErrors:    errorVMs,
	}

	templates.Render(w, r, "ledger/stats", data)
}

// HandleDelete handles POST /ledger/{id}/delete - delete a single entry.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := ledgerstore.New(h.DB)
	entry, err := store.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Delete by request ID
	_, err = store.DeleteByRequestIDs(ctx, []string{entry.RequestID})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Log.Info("ledger entry deleted",
		zap.String("request_id", entry.RequestID))

	// Return empty response with HX-Redirect
	w.Header().Set("HX-Redirect", "/ledger")
	w.WriteHeader(http.StatusOK)
}

// HandleDeleteRange handles POST /ledger/delete-range - delete entries by date range.
func (h *Handler) HandleDeleteRange(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	startStr := r.FormValue("start_date")
	endStr := r.FormValue("end_date")

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		http.Error(w, "Invalid start date", http.StatusBadRequest)
		return
	}

	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		http.Error(w, "Invalid end date", http.StatusBadRequest)
		return
	}
	end = end.Add(24*time.Hour - time.Second) // End of day

	store := ledgerstore.New(h.DB)
	count, err := store.DeleteByDateRange(ctx, start, end)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Log.Info("ledger entries deleted by date range",
		zap.Time("start", start),
		zap.Time("end", end),
		zap.Int64("count", count))

	// Return success message
	w.Header().Set("HX-Redirect", "/ledger")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Deleted %d entries", count)
}

// toLedgerEntryVM converts a store Entry to a view model.
func toLedgerEntryVM(e ledgerstore.Entry) LedgerEntryVM {
	return LedgerEntryVM{
		ID:                 e.ID.Hex(),
		RequestID:          e.RequestID,
		TraceID:            e.TraceID,
		ClientRequestID:    e.ClientRequestID,
		Method:             e.Method,
		Path:               e.Path,
		Query:              e.Query,
		Headers:            e.Headers,
		RemoteIP:           e.RemoteIP,
		ActorType:          e.ActorType,
		ActorID:            e.ActorID,
		ActorName:          e.ActorName,
		RequestBodySize:    e.RequestBodySize,
		RequestBodyHash:    e.RequestBodyHash,
		RequestBodyPreview: e.RequestBodyPreview,
		RequestContentType: e.RequestContentType,
		StatusCode:         e.StatusCode,
		ResponseSize:       e.ResponseSize,
		ErrorClass:         e.ErrorClass,
		ErrorMessage:       e.ErrorMessage,
		DecodeMs:           e.Timing.DecodeMs,
		ValidateMs:         e.Timing.ValidateMs,
		DBQueryMs:          e.Timing.DBQueryMs,
		EncodeMs:           e.Timing.EncodeMs,
		TotalMs:            e.Timing.TotalMs,
		StartedAt:          e.StartedAt.Format("2006-01-02 15:04:05"),
		CompletedAt:        e.CompletedAt.Format("2006-01-02 15:04:05"),
		Duration:           fmt.Sprintf("%.2fms", e.Timing.TotalMs),
		Metadata:           e.Metadata,
		StatusClass:        getStatusClass(e.StatusCode),
	}
}

// getStatusClass returns a CSS class based on status code.
func getStatusClass(code int) string {
	switch {
	case code >= 500:
		return "text-red-600 dark:text-red-400"
	case code >= 400:
		return "text-yellow-600 dark:text-yellow-400"
	case code >= 300:
		return "text-blue-600 dark:text-blue-400"
	case code >= 200:
		return "text-green-600 dark:text-green-400"
	default:
		return "text-gray-600 dark:text-gray-400"
	}
}

// internal/app/features/jobs/handler.go
package jobsfeature

import (
	"context"
	"net/http"
	"strconv"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	jobstore "github.com/dalemusser/stratasave/internal/app/store/jobs"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler handles job monitoring HTTP requests.
type Handler struct {
	DB     *mongo.Database
	ErrLog *errorsfeature.ErrorLogger
	Log    *zap.Logger
}

// NewHandler creates a new jobs handler.
func NewHandler(db *mongo.Database, errLog *errorsfeature.ErrorLogger, logger *zap.Logger) *Handler {
	return &Handler{
		DB:     db,
		ErrLog: errLog,
		Log:    logger,
	}
}

// ServeDashboard handles GET /jobs - job monitoring dashboard.
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	store := jobstore.New(h.DB)

	// Get queue stats
	queueStats, err := store.GetAllQueueStats(ctx)
	if err != nil {
		h.ErrLog.Log(r, "failed to load queue stats", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get recent failed jobs
	recentFailed, err := store.RecentFailed(ctx, 10)
	if err != nil {
		h.ErrLog.Log(r, "failed to load recent failures", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Convert to view models
	statsVMs := make([]QueueStatsVM, len(queueStats))
	for i, s := range queueStats {
		statsVMs[i] = toQueueStatsVM(s)
	}

	failedVMs := make([]JobVM, len(recentFailed))
	for i, j := range recentFailed {
		failedVMs[i] = toJobVM(j)
	}

	base := viewdata.NewBaseVM(r, h.DB, "Job Queue", "/dashboard")
	data := JobDashboardVM{
		BaseVM:       base,
		QueueStats:   statsVMs,
		RecentFailed: failedVMs,
	}

	templates.Render(w, r, "jobs/dashboard", data)
}

// ServeList handles GET /jobs/list - list jobs with filtering.
func (h *Handler) ServeList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	filter := jobstore.ListFilter{
		QueueName: r.URL.Query().Get("queue"),
		JobType:   r.URL.Query().Get("type"),
		Status:    r.URL.Query().Get("status"),
	}

	store := jobstore.New(h.DB)
	result, err := store.List(ctx, filter, page, 50)
	if err != nil {
		h.ErrLog.Log(r, "failed to load jobs", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	jobVMs := make([]JobVM, len(result.Jobs))
	for i, j := range result.Jobs {
		jobVMs[i] = toJobVM(j)
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

	base := viewdata.NewBaseVM(r, h.DB, "All Jobs", "/jobs")
	data := JobListVM{
		BaseVM:     base,
		Jobs:       jobVMs,
		Filter:     filter,
		Page:       result.Page,
		TotalPages: result.TotalPages,
		TotalCount: result.TotalCount,
		PrevPage:   prevPage,
		NextPage:   nextPage,
	}

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "jobs-table" {
		templates.RenderSnippet(w, "jobs_table", data)
		return
	}

	templates.Render(w, r, "jobs/list", data)
}

// ServeDetail handles GET /jobs/{id} - view job details.
func (h *Handler) ServeDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := jobstore.New(h.DB)
	job, err := store.GetByID(ctx, id)
	if err != nil {
		if err == jobstore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to load job", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	base := viewdata.NewBaseVM(r, h.DB, "Job Details", "/jobs/list")
	data := JobDetailVM{
		BaseVM: base,
		Job:    toJobVM(*job),
	}

	templates.Render(w, r, "jobs/detail", data)
}

// HandleRetry handles POST /jobs/{id}/retry - retry a failed job.
func (h *Handler) HandleRetry(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := jobstore.New(h.DB)
	err = store.Retry(ctx, id)
	if err != nil {
		if err == jobstore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to retry job", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Log.Info("job retried", zap.String("job_id", idStr))

	w.Header().Set("HX-Redirect", "/jobs/"+idStr)
	w.WriteHeader(http.StatusOK)
}

// HandleCancel handles POST /jobs/{id}/cancel - cancel a pending/running job.
func (h *Handler) HandleCancel(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := jobstore.New(h.DB)
	err = store.Cancel(ctx, id)
	if err != nil {
		if err == jobstore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to cancel job", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Log.Info("job cancelled", zap.String("job_id", idStr))

	w.Header().Set("HX-Redirect", "/jobs/"+idStr)
	w.WriteHeader(http.StatusOK)
}

// toQueueStatsVM converts store QueueStats to a view model.
func toQueueStatsVM(s jobstore.QueueStats) QueueStatsVM {
	vm := QueueStatsVM{
		QueueName: s.QueueName,
		Pending:   s.Pending,
		Running:   s.Running,
		Completed: s.Completed,
		Failed:    s.Failed,
		Cancelled: s.Cancelled,
		Total:     s.TotalJobs,
	}
	if s.OldestPending != nil {
		vm.OldestPending = s.OldestPending.Format("2006-01-02 15:04:05")
	}
	return vm
}

// toJobVM converts a store Job to a view model.
func toJobVM(j jobstore.Job) JobVM {
	vm := JobVM{
		ID:          j.ID.Hex(),
		QueueName:   j.QueueName,
		JobType:     j.JobType,
		Payload:     j.Payload,
		Status:      j.Status,
		Priority:    j.Priority,
		Attempts:    j.Attempts,
		MaxAttempts: j.MaxAttempts,
		Error:       j.Error,
		Result:      j.Result,
		ScheduledAt: j.ScheduledAt.Format("2006-01-02 15:04:05"),
		CreatedAt:   j.CreatedAt.Format("2006-01-02 15:04:05"),
		StatusClass: getStatusClass(j.Status),
	}

	if j.StartedAt != nil {
		vm.StartedAt = j.StartedAt.Format("2006-01-02 15:04:05")
	}
	if j.CompletedAt != nil {
		vm.CompletedAt = j.CompletedAt.Format("2006-01-02 15:04:05")
	}

	return vm
}

// getStatusClass returns a CSS class based on job status.
func getStatusClass(status string) string {
	switch status {
	case jobstore.StatusPending:
		return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-400"
	case jobstore.StatusRunning:
		return "bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-400"
	case jobstore.StatusCompleted:
		return "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-400"
	case jobstore.StatusFailed:
		return "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-400"
	case jobstore.StatusCancelled:
		return "bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-300"
	default:
		return "bg-gray-100 text-gray-700 dark:bg-gray-600 dark:text-gray-300"
	}
}

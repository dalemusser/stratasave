// internal/app/features/jobs/types.go
package jobsfeature

import (
	jobstore "github.com/dalemusser/stratasave/internal/app/store/jobs"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
)

// QueueStatsVM is the view model for queue statistics.
type QueueStatsVM struct {
	QueueName     string
	Pending       int64
	Running       int64
	Completed     int64
	Failed        int64
	Cancelled     int64
	Total         int64
	OldestPending string
}

// JobVM is the view model for a single job.
type JobVM struct {
	ID          string
	QueueName   string
	JobType     string
	Payload     map[string]any
	Status      string
	Priority    int
	Attempts    int
	MaxAttempts int
	Error       string
	Result      map[string]any
	ScheduledAt string
	StartedAt   string
	CompletedAt string
	CreatedAt   string
	StatusClass string // CSS class for status badge
}

// JobDashboardVM is the view model for the jobs dashboard page.
type JobDashboardVM struct {
	viewdata.BaseVM
	QueueStats   []QueueStatsVM
	RecentFailed []JobVM
}

// JobListVM is the view model for the jobs list page.
type JobListVM struct {
	viewdata.BaseVM
	Jobs       []JobVM
	Filter     jobstore.ListFilter
	Page       int
	TotalPages int
	TotalCount int64
	PrevPage   int
	NextPage   int
}

// JobDetailVM is the view model for the job detail page.
type JobDetailVM struct {
	viewdata.BaseVM
	Job JobVM
}

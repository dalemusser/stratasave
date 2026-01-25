// internal/app/store/jobs/jobstore.go
package jobstore

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Job status constants.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Job represents a background job.
type Job struct {
	ID          primitive.ObjectID `bson:"_id"`
	QueueName   string             `bson:"queue_name"`   // "email", "export", "cleanup"
	JobType     string             `bson:"job_type"`     // "send_email", "generate_report"
	Payload     map[string]any     `bson:"payload"`      // Job-specific data
	Status      string             `bson:"status"`       // pending, running, completed, failed
	Priority    int                `bson:"priority"`     // Higher = sooner
	Attempts    int                `bson:"attempts"`     // Current attempt count
	MaxAttempts int                `bson:"max_attempts"` // Maximum retry attempts
	Error       string             `bson:"error,omitempty"`
	Result      map[string]any     `bson:"result,omitempty"`
	ScheduledAt time.Time          `bson:"scheduled_at"`          // When to run (for delayed jobs)
	StartedAt   *time.Time         `bson:"started_at,omitempty"`  // When processing started
	CompletedAt *time.Time         `bson:"completed_at,omitempty"`// When processing finished
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	WorkerID    string             `bson:"worker_id,omitempty"` // ID of worker processing this job
}

var (
	// ErrNotFound is returned when a job is not found.
	ErrNotFound = errors.New("job not found")
	// ErrAlreadyProcessing is returned when attempting to claim a job that's already being processed.
	ErrAlreadyProcessing = errors.New("job is already being processed")
)

// Store provides job persistence.
type Store struct {
	c *mongo.Collection
}

// New creates a new job store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("jobs")}
}

// CreateInput holds the fields for creating a new job.
type CreateInput struct {
	QueueName   string
	JobType     string
	Payload     map[string]any
	Priority    int
	MaxAttempts int
	ScheduledAt *time.Time // nil = run immediately
}

// Create creates a new job.
func (s *Store) Create(ctx context.Context, input CreateInput) (Job, error) {
	now := time.Now()

	scheduledAt := now
	if input.ScheduledAt != nil {
		scheduledAt = *input.ScheduledAt
	}

	maxAttempts := input.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 3
	}

	job := Job{
		ID:          primitive.NewObjectID(),
		QueueName:   input.QueueName,
		JobType:     input.JobType,
		Payload:     input.Payload,
		Status:      StatusPending,
		Priority:    input.Priority,
		Attempts:    0,
		MaxAttempts: maxAttempts,
		ScheduledAt: scheduledAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if _, err := s.c.InsertOne(ctx, job); err != nil {
		return Job{}, err
	}

	return job, nil
}

// Enqueue is a convenience method to create a job that runs immediately.
func (s *Store) Enqueue(ctx context.Context, queueName, jobType string, payload map[string]any) (Job, error) {
	return s.Create(ctx, CreateInput{
		QueueName: queueName,
		JobType:   jobType,
		Payload:   payload,
	})
}

// EnqueueDelayed creates a job that runs after the specified delay.
func (s *Store) EnqueueDelayed(ctx context.Context, queueName, jobType string, payload map[string]any, delay time.Duration) (Job, error) {
	scheduledAt := time.Now().Add(delay)
	return s.Create(ctx, CreateInput{
		QueueName:   queueName,
		JobType:     jobType,
		Payload:     payload,
		ScheduledAt: &scheduledAt,
	})
}

// EnqueueAt creates a job that runs at the specified time.
func (s *Store) EnqueueAt(ctx context.Context, queueName, jobType string, payload map[string]any, scheduledAt time.Time) (Job, error) {
	return s.Create(ctx, CreateInput{
		QueueName:   queueName,
		JobType:     jobType,
		Payload:     payload,
		ScheduledAt: &scheduledAt,
	})
}

// ClaimNext atomically claims the next available job for processing.
// Returns nil, nil if no jobs are available.
func (s *Store) ClaimNext(ctx context.Context, queueName, workerID string) (*Job, error) {
	now := time.Now()

	filter := bson.M{
		"queue_name":   queueName,
		"status":       StatusPending,
		"scheduled_at": bson.M{"$lte": now},
	}

	update := bson.M{
		"$set": bson.M{
			"status":     StatusRunning,
			"started_at": now,
			"worker_id":  workerID,
			"updated_at": now,
		},
		"$inc": bson.M{
			"attempts": 1,
		},
	}

	opts := options.FindOneAndUpdate().
		SetSort(bson.D{
			{Key: "priority", Value: -1},  // Highest priority first
			{Key: "scheduled_at", Value: 1}, // Then oldest scheduled
		}).
		SetReturnDocument(options.After)

	var job Job
	err := s.c.FindOneAndUpdate(ctx, filter, update, opts).Decode(&job)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &job, nil
}

// Complete marks a job as completed with optional result data.
func (s *Store) Complete(ctx context.Context, id primitive.ObjectID, result map[string]any) error {
	now := time.Now()
	_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"status":       StatusCompleted,
			"completed_at": now,
			"result":       result,
			"updated_at":   now,
		},
	})
	return err
}

// Fail marks a job as failed with an error message.
// If the job has remaining attempts, it will be rescheduled.
func (s *Store) Fail(ctx context.Context, id primitive.ObjectID, errMsg string, retryDelay time.Duration) error {
	// First get the job to check attempts
	job, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now()

	// If we have remaining attempts, reschedule
	if job.Attempts < job.MaxAttempts {
		_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
			"$set": bson.M{
				"status":       StatusPending,
				"error":        errMsg,
				"scheduled_at": now.Add(retryDelay),
				"started_at":   nil,
				"worker_id":    "",
				"updated_at":   now,
			},
		})
		return err
	}

	// No more attempts - mark as failed
	_, err = s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"status":       StatusFailed,
			"error":        errMsg,
			"completed_at": now,
			"updated_at":   now,
		},
	})
	return err
}

// Cancel cancels a pending or running job.
func (s *Store) Cancel(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	result, err := s.c.UpdateOne(ctx, bson.M{
		"_id":    id,
		"status": bson.M{"$in": []string{StatusPending, StatusRunning}},
	}, bson.M{
		"$set": bson.M{
			"status":       StatusCancelled,
			"completed_at": now,
			"updated_at":   now,
		},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// Retry retries a failed or cancelled job.
func (s *Store) Retry(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	result, err := s.c.UpdateOne(ctx, bson.M{
		"_id":    id,
		"status": bson.M{"$in": []string{StatusFailed, StatusCancelled}},
	}, bson.M{
		"$set": bson.M{
			"status":       StatusPending,
			"scheduled_at": now,
			"started_at":   nil,
			"completed_at": nil,
			"worker_id":    "",
			"error":        "",
			"updated_at":   now,
		},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID retrieves a job by ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*Job, error) {
	var job Job
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&job); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &job, nil
}

// ListFilter specifies criteria for listing jobs.
type ListFilter struct {
	QueueName string
	JobType   string
	Status    string
	Statuses  []string // Multiple statuses
}

// ListResult contains a page of jobs with pagination info.
type ListResult struct {
	Jobs       []Job
	TotalCount int64
	Page       int
	PageSize   int
	TotalPages int
}

// List returns jobs matching the filter with pagination.
func (s *Store) List(ctx context.Context, filter ListFilter, page, pageSize int) (ListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := s.buildQuery(filter)

	// Count total
	total, err := s.c.CountDocuments(ctx, query)
	if err != nil {
		return ListResult{}, err
	}

	// Calculate pagination
	skip := (page - 1) * pageSize
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	// Find jobs
	opts := options.Find().
		SetSort(bson.D{
			{Key: "created_at", Value: -1},
		}).
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize))

	cur, err := s.c.Find(ctx, query, opts)
	if err != nil {
		return ListResult{}, err
	}
	defer cur.Close(ctx)

	var jobs []Job
	if err := cur.All(ctx, &jobs); err != nil {
		return ListResult{}, err
	}

	return ListResult{
		Jobs:       jobs,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// buildQuery constructs a MongoDB query from ListFilter.
func (s *Store) buildQuery(filter ListFilter) bson.M {
	query := bson.M{}

	if filter.QueueName != "" {
		query["queue_name"] = filter.QueueName
	}
	if filter.JobType != "" {
		query["job_type"] = filter.JobType
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	} else if len(filter.Statuses) > 0 {
		query["status"] = bson.M{"$in": filter.Statuses}
	}

	return query
}

// QueueStats holds statistics for a queue.
type QueueStats struct {
	QueueName     string
	Pending       int64
	Running       int64
	Completed     int64
	Failed        int64
	Cancelled     int64
	TotalJobs     int64
	OldestPending *time.Time
}

// GetQueueStats returns statistics for a queue.
func (s *Store) GetQueueStats(ctx context.Context, queueName string) (QueueStats, error) {
	stats := QueueStats{QueueName: queueName}

	filter := bson.M{}
	if queueName != "" {
		filter["queue_name"] = queueName
	}

	// Get counts by status
	pipeline := []bson.M{
		{"$match": filter},
		{
			"$group": bson.M{
				"_id":   "$status",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cur, err := s.c.Aggregate(ctx, pipeline)
	if err != nil {
		return stats, err
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var result struct {
			Status string `bson:"_id"`
			Count  int64  `bson:"count"`
		}
		if err := cur.Decode(&result); err != nil {
			continue
		}

		switch result.Status {
		case StatusPending:
			stats.Pending = result.Count
		case StatusRunning:
			stats.Running = result.Count
		case StatusCompleted:
			stats.Completed = result.Count
		case StatusFailed:
			stats.Failed = result.Count
		case StatusCancelled:
			stats.Cancelled = result.Count
		}
		stats.TotalJobs += result.Count
	}

	// Get oldest pending job
	var oldestJob Job
	opts := options.FindOne().SetSort(bson.D{{Key: "scheduled_at", Value: 1}})
	pendingFilter := bson.M{"status": StatusPending}
	if queueName != "" {
		pendingFilter["queue_name"] = queueName
	}
	if err := s.c.FindOne(ctx, pendingFilter, opts).Decode(&oldestJob); err == nil {
		stats.OldestPending = &oldestJob.ScheduledAt
	}

	return stats, nil
}

// GetAllQueueStats returns statistics for all queues.
func (s *Store) GetAllQueueStats(ctx context.Context) ([]QueueStats, error) {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id": bson.M{
					"queue":  "$queue_name",
					"status": "$status",
				},
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$group": bson.M{
				"_id": "$_id.queue",
				"statuses": bson.M{
					"$push": bson.M{
						"status": "$_id.status",
						"count":  "$count",
					},
				},
			},
		},
	}

	cur, err := s.c.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results []QueueStats
	for cur.Next(ctx) {
		var doc struct {
			QueueName string `bson:"_id"`
			Statuses  []struct {
				Status string `bson:"status"`
				Count  int64  `bson:"count"`
			} `bson:"statuses"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}

		stats := QueueStats{QueueName: doc.QueueName}
		for _, s := range doc.Statuses {
			switch s.Status {
			case StatusPending:
				stats.Pending = s.Count
			case StatusRunning:
				stats.Running = s.Count
			case StatusCompleted:
				stats.Completed = s.Count
			case StatusFailed:
				stats.Failed = s.Count
			case StatusCancelled:
				stats.Cancelled = s.Count
			}
			stats.TotalJobs += s.Count
		}
		results = append(results, stats)
	}

	return results, nil
}

// DeleteOlderThan deletes completed/cancelled jobs older than the cutoff.
func (s *Store) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.c.DeleteMany(ctx, bson.M{
		"status":       bson.M{"$in": []string{StatusCompleted, StatusCancelled}},
		"completed_at": bson.M{"$lt": cutoff},
	})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// RecentFailed returns the most recent failed jobs.
func (s *Store) RecentFailed(ctx context.Context, limit int) ([]Job, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "completed_at", Value: -1}}).
		SetLimit(int64(limit))

	cur, err := s.c.Find(ctx, bson.M{"status": StatusFailed}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var jobs []Job
	if err := cur.All(ctx, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// CleanupStaleRunning marks jobs that have been running too long as failed.
// This handles jobs that were claimed by workers that crashed.
func (s *Store) CleanupStaleRunning(ctx context.Context, staleThreshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-staleThreshold)
	now := time.Now()

	result, err := s.c.UpdateMany(ctx, bson.M{
		"status":     StatusRunning,
		"started_at": bson.M{"$lt": cutoff},
	}, bson.M{
		"$set": bson.M{
			"status":     StatusPending, // Re-queue for retry
			"started_at": nil,
			"worker_id":  "",
			"error":      "worker timeout - job re-queued",
			"updated_at": now,
		},
	})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

// internal/app/system/jobrunner/runner.go
package jobrunner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/jobs"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// JobHandler processes a job and returns a result or error.
type JobHandler func(ctx context.Context, payload map[string]any) (map[string]any, error)

// Config holds configuration for the job runner.
type Config struct {
	// WorkerCount is the number of concurrent workers per queue.
	WorkerCount int

	// PollInterval is how often to poll for new jobs.
	PollInterval time.Duration

	// RetryDelay is the base delay before retrying a failed job.
	// Actual delay is RetryDelay * attempts (exponential backoff).
	RetryDelay time.Duration

	// StaleJobThreshold is how long a job can be "running" before it's considered stale.
	// Stale jobs are re-queued automatically.
	StaleJobThreshold time.Duration

	// CleanupInterval is how often to run cleanup tasks.
	CleanupInterval time.Duration

	// JobRetention is how long to keep completed/cancelled jobs.
	JobRetention time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		WorkerCount:       3,
		PollInterval:      time.Second,
		RetryDelay:        5 * time.Second,
		StaleJobThreshold: 5 * time.Minute,
		CleanupInterval:   time.Hour,
		JobRetention:      7 * 24 * time.Hour, // 7 days
	}
}

// Runner manages job processing across multiple queues.
type Runner struct {
	store    *jobstore.Store
	handlers map[string]JobHandler
	config   Config
	logger   *zap.Logger

	workerID   string
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    atomic.Int32
	activeJobs sync.Map // jobID -> struct{}

	mu      sync.RWMutex
	queues  map[string]bool // Registered queue names
	started bool
}

// New creates a new job runner.
func New(store *jobstore.Store, logger *zap.Logger, config ...Config) *Runner {
	cfg := DefaultConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return &Runner{
		store:    store,
		handlers: make(map[string]JobHandler),
		config:   cfg,
		logger:   logger,
		workerID: uuid.New().String()[:8],
		queues:   make(map[string]bool),
	}
}

// Register registers a handler for a job type.
func (r *Runner) Register(jobType string, handler JobHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[jobType] = handler
}

// AddQueue registers a queue name for processing.
func (r *Runner) AddQueue(queueName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queues[queueName] = true
}

// Start begins processing jobs on all registered queues.
func (r *Runner) Start() error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return fmt.Errorf("runner already started")
	}
	r.started = true

	queues := make([]string, 0, len(r.queues))
	for q := range r.queues {
		queues = append(queues, q)
	}
	r.mu.Unlock()

	if len(queues) == 0 {
		r.logger.Warn("job runner started with no queues registered")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	// Start workers for each queue
	for _, queueName := range queues {
		for i := 0; i < r.config.WorkerCount; i++ {
			r.wg.Add(1)
			workerName := fmt.Sprintf("%s-%s-%d", r.workerID, queueName, i)
			go r.worker(ctx, queueName, workerName)
		}
	}

	// Start cleanup goroutine
	r.wg.Add(1)
	go r.cleanup(ctx)

	r.logger.Info("job runner started",
		zap.Int("queues", len(queues)),
		zap.Int("workers_per_queue", r.config.WorkerCount),
		zap.Strings("queue_names", queues))

	return nil
}

// Stop gracefully stops the runner and waits for active jobs to complete.
func (r *Runner) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	if r.cancel != nil {
		r.cancel()
	}

	// Wait for workers to complete with timeout
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("job runner stopped gracefully")
		return nil
	case <-ctx.Done():
		// Log active jobs
		var activeJobs []string
		r.activeJobs.Range(func(key, _ any) bool {
			activeJobs = append(activeJobs, key.(string))
			return true
		})
		r.logger.Warn("job runner shutdown timed out",
			zap.Int32("active_jobs", r.running.Load()),
			zap.Strings("job_ids", activeJobs))
		return ctx.Err()
	}
}

// worker processes jobs from a single queue.
func (r *Runner) worker(ctx context.Context, queueName, workerName string) {
	defer r.wg.Done()

	r.logger.Debug("worker started",
		zap.String("worker", workerName),
		zap.String("queue", queueName))

	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Debug("worker stopping",
				zap.String("worker", workerName))
			return
		case <-ticker.C:
			r.processNextJob(ctx, queueName, workerName)
		}
	}
}

// processNextJob claims and processes the next available job.
func (r *Runner) processNextJob(ctx context.Context, queueName, workerName string) {
	// Claim next job
	claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	job, err := r.store.ClaimNext(claimCtx, queueName, workerName)
	cancel()

	if err != nil {
		if ctx.Err() == nil {
			r.logger.Error("failed to claim job",
				zap.String("queue", queueName),
				zap.Error(err))
		}
		return
	}

	if job == nil {
		return // No jobs available
	}

	// Track active job
	r.running.Add(1)
	r.activeJobs.Store(job.ID.Hex(), struct{}{})
	defer func() {
		r.running.Add(-1)
		r.activeJobs.Delete(job.ID.Hex())
	}()

	// Get handler
	r.mu.RLock()
	handler, ok := r.handlers[job.JobType]
	r.mu.RUnlock()

	if !ok {
		r.logger.Error("no handler registered for job type",
			zap.String("job_type", job.JobType),
			zap.String("job_id", job.ID.Hex()))
		// Fail the job
		failCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.store.Fail(failCtx, job.ID, fmt.Sprintf("no handler for job type: %s", job.JobType), r.config.RetryDelay)
		cancel()
		return
	}

	// Execute handler
	start := time.Now()
	r.logger.Debug("processing job",
		zap.String("job_id", job.ID.Hex()),
		zap.String("job_type", job.JobType),
		zap.Int("attempt", job.Attempts))

	// Create job context with timeout
	jobCtx, jobCancel := context.WithTimeout(ctx, r.config.StaleJobThreshold)
	result, err := handler(jobCtx, job.Payload)
	jobCancel()

	duration := time.Since(start)

	if err != nil {
		retryDelay := r.config.RetryDelay * time.Duration(job.Attempts)

		r.logger.Warn("job failed",
			zap.String("job_id", job.ID.Hex()),
			zap.String("job_type", job.JobType),
			zap.Int("attempt", job.Attempts),
			zap.Int("max_attempts", job.MaxAttempts),
			zap.Duration("duration", duration),
			zap.Error(err))

		// Mark job as failed
		failCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if failErr := r.store.Fail(failCtx, job.ID, err.Error(), retryDelay); failErr != nil {
			r.logger.Error("failed to mark job as failed",
				zap.String("job_id", job.ID.Hex()),
				zap.Error(failErr))
		}
		cancel()
		return
	}

	r.logger.Info("job completed",
		zap.String("job_id", job.ID.Hex()),
		zap.String("job_type", job.JobType),
		zap.Duration("duration", duration))

	// Mark job as completed
	completeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := r.store.Complete(completeCtx, job.ID, result); err != nil {
		r.logger.Error("failed to mark job as completed",
			zap.String("job_id", job.ID.Hex()),
			zap.Error(err))
	}
	cancel()
}

// cleanup runs periodic cleanup tasks.
func (r *Runner) cleanup(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()

	// Run cleanup immediately on start
	r.runCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runCleanup(ctx)
		}
	}
}

// runCleanup performs cleanup tasks.
func (r *Runner) runCleanup(ctx context.Context) {
	// Cleanup stale running jobs
	staleCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	count, err := r.store.CleanupStaleRunning(staleCtx, r.config.StaleJobThreshold)
	cancel()
	if err != nil {
		r.logger.Error("failed to cleanup stale jobs", zap.Error(err))
	} else if count > 0 {
		r.logger.Info("cleaned up stale running jobs", zap.Int64("count", count))
	}

	// Delete old completed jobs
	cutoff := time.Now().Add(-r.config.JobRetention)
	deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	deleted, err := r.store.DeleteOlderThan(deleteCtx, cutoff)
	cancel()
	if err != nil {
		r.logger.Error("failed to delete old jobs", zap.Error(err))
	} else if deleted > 0 {
		r.logger.Info("deleted old completed jobs", zap.Int64("count", deleted))
	}
}

// Enqueue adds a job to be processed.
func (r *Runner) Enqueue(ctx context.Context, queueName, jobType string, payload map[string]any) (jobstore.Job, error) {
	return r.store.Enqueue(ctx, queueName, jobType, payload)
}

// EnqueueDelayed adds a job to be processed after a delay.
func (r *Runner) EnqueueDelayed(ctx context.Context, queueName, jobType string, payload map[string]any, delay time.Duration) (jobstore.Job, error) {
	return r.store.EnqueueDelayed(ctx, queueName, jobType, payload, delay)
}

// EnqueueAt adds a job to be processed at a specific time.
func (r *Runner) EnqueueAt(ctx context.Context, queueName, jobType string, payload map[string]any, at time.Time) (jobstore.Job, error) {
	return r.store.EnqueueAt(ctx, queueName, jobType, payload, at)
}

// Stats returns current runner statistics.
type Stats struct {
	WorkerID    string
	ActiveJobs  int32
	QueueStats  []jobstore.QueueStats
}

// Stats returns current runner statistics.
func (r *Runner) Stats(ctx context.Context) (Stats, error) {
	queueStats, err := r.store.GetAllQueueStats(ctx)
	if err != nil {
		return Stats{}, err
	}

	return Stats{
		WorkerID:   r.workerID,
		ActiveJobs: r.running.Load(),
		QueueStats: queueStats,
	}, nil
}

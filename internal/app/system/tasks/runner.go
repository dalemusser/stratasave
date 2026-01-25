// internal/app/system/tasks/runner.go
package tasks

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Job represents a scheduled background task.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Runner manages background job execution.
type Runner struct {
	logger   *zap.Logger
	jobs     []Job
	wg       sync.WaitGroup
	cancel   context.CancelFunc
	running  atomic.Int32 // Count of currently executing jobs
	jobNames sync.Map     // Track which jobs are currently running
}

// New creates a new task runner.
func New(logger *zap.Logger) *Runner {
	return &Runner{
		logger: logger,
	}
}

// Register adds a job to the runner.
func (r *Runner) Register(job Job) {
	r.jobs = append(r.jobs, job)
}

// Start begins executing all registered jobs.
// Call Stop to gracefully shutdown.
func (r *Runner) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	for _, job := range r.jobs {
		r.wg.Add(1)
		go r.runJob(ctx, job)
	}

	r.logger.Info("background task runner started",
		zap.Int("job_count", len(r.jobs)))
}

// Stop gracefully stops all running jobs within the given context's deadline.
// If ctx is cancelled before all jobs complete, it returns ctx.Err().
// Pass context.Background() for unlimited wait time.
func (r *Runner) Stop(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}

	// Wait for jobs to complete with timeout
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("background task runner stopped gracefully")
		return nil
	case <-ctx.Done():
		// Log which jobs are still running
		var stillRunning []string
		r.jobNames.Range(func(key, _ any) bool {
			stillRunning = append(stillRunning, key.(string))
			return true
		})
		r.logger.Warn("background task runner shutdown timed out",
			zap.Strings("jobs_still_running", stillRunning),
			zap.Int32("running_count", r.running.Load()))
		return ctx.Err()
	}
}

// runJob executes a single job on its interval.
func (r *Runner) runJob(ctx context.Context, job Job) {
	defer r.wg.Done()

	// Run immediately on startup
	r.executeJob(ctx, job)

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Debug("job stopped", zap.String("job", job.Name))
			return
		case <-ticker.C:
			r.executeJob(ctx, job)
		}
	}
}

// executeJob runs a job and logs the result.
func (r *Runner) executeJob(ctx context.Context, job Job) {
	// Track that this job is running
	r.running.Add(1)
	r.jobNames.Store(job.Name, struct{}{})
	defer func() {
		r.running.Add(-1)
		r.jobNames.Delete(job.Name)
	}()

	start := time.Now()
	r.logger.Debug("job starting", zap.String("job", job.Name))

	if err := job.Run(ctx); err != nil {
		// Don't log context cancellation as an error during shutdown
		if ctx.Err() != nil {
			r.logger.Debug("job cancelled during shutdown",
				zap.String("job", job.Name),
				zap.Duration("duration", time.Since(start)))
			return
		}
		r.logger.Error("job failed",
			zap.String("job", job.Name),
			zap.Duration("duration", time.Since(start)),
			zap.Error(err))
		return
	}

	r.logger.Debug("job completed",
		zap.String("job", job.Name),
		zap.Duration("duration", time.Since(start)))
}

// RunOnce executes a job immediately (useful for testing or manual triggers).
func (r *Runner) RunOnce(ctx context.Context, name string) error {
	for _, job := range r.jobs {
		if job.Name == name {
			return job.Run(ctx)
		}
	}
	return nil
}

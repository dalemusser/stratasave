package tasks_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/tasks"
	"go.uber.org/zap"
)

func TestRunner_StartAndStop(t *testing.T) {
	logger := zap.NewNop()
	runner := tasks.New(logger)

	var runCount atomic.Int32
	runner.Register(tasks.Job{
		Name:     "test-job",
		Interval: 100 * time.Millisecond,
		Run: func(ctx context.Context) error {
			runCount.Add(1)
			return nil
		},
	})

	runner.Start()

	// Wait for at least one execution
	time.Sleep(50 * time.Millisecond)

	// Stop with generous timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}

	// Job should have run at least once (runs immediately on start)
	if runCount.Load() < 1 {
		t.Errorf("expected job to run at least once, ran %d times", runCount.Load())
	}
}

func TestRunner_StopWithTimeout(t *testing.T) {
	logger := zap.NewNop()
	runner := tasks.New(logger)

	inSleep := make(chan struct{})
	runner.Register(tasks.Job{
		Name:     "slow-job",
		Interval: 1 * time.Hour, // Won't repeat during test
		Run: func(ctx context.Context) error {
			// Signal that we're about to sleep
			close(inSleep)
			// Simulate a long-running job that IGNORES context (bad behavior)
			// This tests that Stop() times out properly
			time.Sleep(5 * time.Second)
			return nil
		},
	})

	runner.Start()

	// Wait for job to enter the sleep
	<-inSleep

	// Give the job a moment to be fully in the sleep
	time.Sleep(10 * time.Millisecond)

	// Stop with very short timeout - should fail because job ignores context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := runner.Stop(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got: %v", err)
	}
}

func TestRunner_GracefulStop(t *testing.T) {
	logger := zap.NewNop()
	runner := tasks.New(logger)

	completed := make(chan struct{})
	runner.Register(tasks.Job{
		Name:     "quick-job",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context) error {
			// Quick job that completes before timeout
			time.Sleep(10 * time.Millisecond)
			close(completed)
			return nil
		},
	})

	runner.Start()

	// Wait for job to complete
	<-completed

	// Stop should succeed quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := runner.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

func TestRunner_MultipleJobs(t *testing.T) {
	logger := zap.NewNop()
	runner := tasks.New(logger)

	var job1Count, job2Count atomic.Int32

	runner.Register(tasks.Job{
		Name:     "job-1",
		Interval: 50 * time.Millisecond,
		Run: func(ctx context.Context) error {
			job1Count.Add(1)
			return nil
		},
	})

	runner.Register(tasks.Job{
		Name:     "job-2",
		Interval: 50 * time.Millisecond,
		Run: func(ctx context.Context) error {
			job2Count.Add(1)
			return nil
		},
	})

	runner.Start()

	// Let jobs run a few times
	time.Sleep(150 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}

	// Both jobs should have run at least once
	if job1Count.Load() < 1 {
		t.Errorf("job-1 should have run at least once, ran %d times", job1Count.Load())
	}
	if job2Count.Load() < 1 {
		t.Errorf("job-2 should have run at least once, ran %d times", job2Count.Load())
	}
}

func TestRunner_RunOnce(t *testing.T) {
	logger := zap.NewNop()
	runner := tasks.New(logger)

	var runCount atomic.Int32
	runner.Register(tasks.Job{
		Name:     "manual-job",
		Interval: 1 * time.Hour, // Long interval so it doesn't auto-run again
		Run: func(ctx context.Context) error {
			runCount.Add(1)
			return nil
		},
	})

	// Don't start the runner - just test RunOnce
	ctx := context.Background()
	err := runner.RunOnce(ctx, "manual-job")
	if err != nil {
		t.Errorf("RunOnce() returned error: %v", err)
	}

	if runCount.Load() != 1 {
		t.Errorf("expected job to run once, ran %d times", runCount.Load())
	}
}

func TestRunner_RunOnce_NotFound(t *testing.T) {
	logger := zap.NewNop()
	runner := tasks.New(logger)

	ctx := context.Background()
	err := runner.RunOnce(ctx, "nonexistent-job")
	if err != nil {
		t.Errorf("RunOnce() for nonexistent job should return nil, got: %v", err)
	}
}

func TestRunner_JobContextCancellation(t *testing.T) {
	logger := zap.NewNop()
	runner := tasks.New(logger)

	contextCancelled := make(chan struct{})
	runner.Register(tasks.Job{
		Name:     "context-aware-job",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context) error {
			// Wait for context cancellation
			<-ctx.Done()
			close(contextCancelled)
			return ctx.Err()
		},
	})

	runner.Start()

	// Give job time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Stop the runner
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}

	// Verify context was cancelled
	select {
	case <-contextCancelled:
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("job context was not cancelled")
	}
}

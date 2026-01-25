// internal/app/bootstrap/shutdown.go
package bootstrap

import (
	"context"

	"github.com/dalemusser/waffle/config"
	"go.uber.org/zap"
)

// Shutdown is an optional hook invoked during WAFFLE's shutdown phase.
//
// This function is called after the HTTP server has stopped accepting new
// requests and existing requests have been drained (or the shutdown timeout
// has elapsed). It is your opportunity to gracefully clean up resources.
//
// The context provided has a timeout (default 10 seconds) and should be
// respected—if cleanup takes too long, the context will be cancelled.
//
// Common uses for Shutdown:
//   - Close database connections
//   - Flush pending writes to external services
//   - Stop background workers gracefully
//   - Close message queue channels
//   - Release file handles or network connections
//
// If an error is returned, it will be logged but won't prevent the process
// from exiting. However, returning nil on success helps ensure clean shutdown
// behavior and accurate logging.
func Shutdown(ctx context.Context, coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) error {
	var firstErr error

	// Stop background task runner with context timeout
	if taskRunner != nil {
		logger.Info("stopping background task runner")
		if err := taskRunner.Stop(ctx); err != nil {
			logger.Warn("background task runner did not stop cleanly", zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// Disconnect MongoDB client
	if deps.MongoClient != nil {
		logger.Info("disconnecting MongoDB client")
		if err := deps.MongoClient.Disconnect(ctx); err != nil {
			logger.Error("MongoDB disconnect failed", zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

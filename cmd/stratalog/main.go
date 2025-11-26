// cmd/stratalog/main.go
package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/dalemusser/stratalog/internal/app/system/config"
	"github.com/dalemusser/stratalog/internal/app/system/indexes"
	"github.com/dalemusser/stratalog/internal/app/system/server"
	"github.com/dalemusser/stratalog/internal/app/system/validators"
	"github.com/dalemusser/stratalog/internal/platform/db"
	"github.com/dalemusser/stratalog/internal/platform/render"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func buildLogger(level string, prod bool) (*zap.Logger, error) {
	var cfg zap.Config
	if prod {
		cfg = zap.NewProductionConfig()
		cfg.Encoding = "json"
	} else {
		cfg = zap.NewDevelopmentConfig()
	}
	// Honor desired level; default to info on bad input.
	if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	// RFC-3339 timestamps.
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return cfg.Build()
}

func buildBootstrapLogger() *zap.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	l, err := cfg.Build()
	if err != nil {
		return zap.NewNop()
	}
	return l
}

func main() {
	// -------------------------------------------------------------------
	// Step 1: Bootstrap logger so early failures are visible.
	// -------------------------------------------------------------------
	bootstrap := buildBootstrapLogger()
	zap.ReplaceGlobals(bootstrap)
	zap.L().Info("Step 1 bootstrap logger initialized", zap.String("encoding", "console"), zap.String("level", "info"))

	// -------------------------------------------------------------------
	// Step 2: Load config
	// -------------------------------------------------------------------
	zap.L().Info("Step 2: loading config…")
	cfg, err := config.Load()
	if err != nil {
		zap.L().Fatal("config load failed", zap.Error(err))
	}
	zap.L().Info("Step 2 complete: config loaded", zap.String("env", cfg.Env), zap.String("log_level", cfg.LogLevel))
	zap.L().Debug("effective config (redacted)", zap.String("config", cfg.Dump()))

	// -------------------------------------------------------------------
	// Step 3: Build final logger
	// -------------------------------------------------------------------
	zap.L().Info("Step 3: building logger…")
	logger, err := buildLogger(cfg.LogLevel, cfg.Env == "prod")
	if err != nil {
		zap.L().Fatal("logger build failed", zap.Error(err))
	}
	defer func() { _ = logger.Sync() }()
	zap.ReplaceGlobals(logger)
	sugar := zap.S()
	sugar.Infow("Step 3 complete: logger initialized", "env", cfg.Env, "level", cfg.LogLevel)

	// -------------------------------------------------------------------
	// Step 4: Connect to MongoDB
	// -------------------------------------------------------------------
	sugar.Infow("Step 4: connecting to MongoDB…", "uri", cfg.MongoURI, "database", cfg.MongoDatabase)
	client, err := db.Connect(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		sugar.Fatalw("MongoDB connection failed", "error", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	}()
	sugar.Infow("Step 4 complete: MongoDB connected", "database", cfg.MongoDatabase)

	// -------------------------------------------------------------------
	// Step 5: Ensure collections & validators (idempotent; skip if unsupported)
	// -------------------------------------------------------------------
	sugar.Infow("Step 5: ensuring collections & validators…", "database", cfg.MongoDatabase)
	{
		bootCtx, cancel := context.WithTimeout(context.Background(), cfg.IndexBootTimeout)
		defer cancel()

		database := client.Database(cfg.MongoDatabase)
		if err := validators.EnsureAll(bootCtx, database); err != nil {
			// This returns an error only for real failures; engines that
			// don't support validators are handled inside EnsureAll.
			sugar.Fatalw("ensuring collections/validators failed", "error", err)
		}
	}
	sugar.Infow("Step 5 complete: collections & validators ensured")

	// -------------------------------------------------------------------
	// Step 6: Ensure MongoDB indexes (idempotent; fail fast if broken)
	// -------------------------------------------------------------------
	sugar.Infow("Step 6: ensuring MongoDB indexes…", "database", cfg.MongoDatabase)
	{
		bootCtx, cancel := context.WithTimeout(context.Background(), cfg.IndexBootTimeout)
		defer cancel()

		database := client.Database(cfg.MongoDatabase)
		if err := indexes.EnsureAll(bootCtx, database); err != nil {
			sugar.Fatalw("ensuring MongoDB indexes failed", "error", err)
		}
	}
	sugar.Infow("Step 6 complete: indexes ensured")

	// -------------------------------------------------------------------
	// Step 7: Boot template engine (must be before starting the server)
	// -------------------------------------------------------------------
	// devMode: true in non-prod so you can add hot-refresh later if you want
	eng := render.New(cfg.Env != "prod")

	if err := eng.Boot(); err != nil {
		sugar.Fatalw("Step 7 template engine boot failed", "error", err)
	}
	render.UseEngine(eng)
	sugar.Infow("Step 7 template engine ready")

	// -------------------------------------------------------------------
	// Step 8: Wire shutdown signals → context
	// -------------------------------------------------------------------
	sugar.Infow("Step 8 wiring shutdown signals")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	// Always listen for Ctrl+C (os.Interrupt). Add SIGTERM on non-Windows.
	if runtime.GOOS == "windows" {
		signal.Notify(sigCh, os.Interrupt)
	} else {
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	}
	go func() {
		sig := <-sigCh
		zap.L().Info("shutdown signal received", zap.Any("signal", sig))
		cancel()
	}()

	// -------------------------------------------------------------------
	// Step 9: Start HTTP server (context-cancellable)
	// -------------------------------------------------------------------
	sugar.Infow("Step 9: starting HTTP server…")
	if err := server.StartServerWithContext(ctx, cfg, client); err != nil {
		sugar.Fatalw("server exited with error", "error", err)
	}
	sugar.Infow("server stopped")
}

// internal/app/bootstrap/db.go
package bootstrap

import (
	"context"
	"fmt"

	"github.com/dalemusser/stratasave/internal/app/system/indexes"
	"github.com/dalemusser/stratasave/internal/app/system/mailer"
	"github.com/dalemusser/stratasave/internal/app/system/seeding"
	"github.com/dalemusser/stratasave/internal/app/system/validators"
	"github.com/dalemusser/waffle/config"
	wafflemongo "github.com/dalemusser/waffle/pantry/mongo"
	"github.com/dalemusser/waffle/pantry/storage"
	"go.uber.org/zap"
)

// ConnectDB connects to databases or other backends.
//
// WAFFLE calls this after configuration is loaded but before EnsureSchema and
// Startup. This is the place to establish connections to:
//   - Databases (MongoDB, PostgreSQL, MySQL, SQLite, etc.)
//   - Caches (Redis, Memcached)
//   - Message queues (RabbitMQ, Kafka)
//   - External services that require persistent connections
//
// Best practices:
//   - Use coreCfg.DBConnectTimeout to set connection timeouts
//   - Log connection attempts and successes for debugging
//   - Return descriptive errors if connections fail
//   - Store clients in the DBDeps struct for use in handlers
func ConnectDB(ctx context.Context, coreCfg *config.CoreConfig, appCfg AppConfig, logger *zap.Logger) (DBDeps, error) {
	// Configure MongoDB connection pool
	poolCfg := wafflemongo.DefaultPoolConfig()
	if appCfg.MongoMaxPoolSize > 0 {
		poolCfg.MaxPoolSize = appCfg.MongoMaxPoolSize
	}
	if appCfg.MongoMinPoolSize > 0 {
		poolCfg.MinPoolSize = appCfg.MongoMinPoolSize
	}

	client, err := wafflemongo.ConnectWithPool(ctx, appCfg.MongoURI, appCfg.MongoDatabase, poolCfg)
	if err != nil {
		return DBDeps{}, err
	}

	db := client.Database(appCfg.MongoDatabase)

	logger.Info("connected to MongoDB",
		zap.String("database", appCfg.MongoDatabase),
		zap.Uint64("max_pool_size", poolCfg.MaxPoolSize),
		zap.Uint64("min_pool_size", poolCfg.MinPoolSize),
	)

	// Initialize file storage
	var store storage.Store
	switch appCfg.StorageType {
	case "s3":
		store, err = storage.NewS3(ctx, storage.S3Config{
			Region:                   appCfg.StorageS3Region,
			Bucket:                   appCfg.StorageS3Bucket,
			Prefix:                   appCfg.StorageS3Prefix,
			CloudFrontURL:            appCfg.StorageCFURL,
			CloudFrontKeyPairID:      appCfg.StorageCFKeyPairID,
			CloudFrontPrivateKeyPath: appCfg.StorageCFKeyPath,
		})
		if err != nil {
			return DBDeps{}, fmt.Errorf("failed to initialize S3 storage: %w", err)
		}
		logger.Info("initialized S3/CloudFront file storage",
			zap.String("bucket", appCfg.StorageS3Bucket),
			zap.String("prefix", appCfg.StorageS3Prefix),
		)
	case "local", "":
		store, err = storage.NewLocal(storage.LocalConfig{
			BasePath: appCfg.StorageLocalPath,
			BaseURL:  appCfg.StorageLocalURL,
		})
		if err != nil {
			return DBDeps{}, fmt.Errorf("failed to initialize local storage: %w", err)
		}
		logger.Info("initialized local file storage",
			zap.String("path", appCfg.StorageLocalPath),
			zap.String("url", appCfg.StorageLocalURL),
		)
	default:
		return DBDeps{}, fmt.Errorf("unknown storage type: %s", appCfg.StorageType)
	}

	// Initialize email mailer
	mail := mailer.New(mailer.Config{
		Host:     appCfg.MailSMTPHost,
		Port:     appCfg.MailSMTPPort,
		User:     appCfg.MailSMTPUser,
		Pass:     appCfg.MailSMTPPass,
		From:     appCfg.MailFrom,
		FromName: appCfg.MailFromName,
	}, logger)
	logger.Info("initialized email mailer",
		zap.String("host", appCfg.MailSMTPHost),
		zap.Int("port", appCfg.MailSMTPPort),
	)

	return DBDeps{
		MongoClient:   client,
		MongoDatabase: db,
		FileStorage:   store,
		Mailer:        mail,
	}, nil
}

// EnsureSchema sets up indexes or schema as needed.
//
// This runs after ConnectDB succeeds but before Startup and before the HTTP
// handler is built. It is optional—if you do not need indexes or migrations,
// you can leave this as a no-op that returns nil.
//
// This is the place to:
//   - Create database indexes for query performance
//   - Run schema migrations
//   - Validate that required collections/tables exist
//   - Set up initial data (seed data, default records)
//
// The context has a timeout based on coreCfg.IndexBootTimeout, so long-running
// migrations should respect context cancellation.
func EnsureSchema(ctx context.Context, coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) error {
	db := deps.MongoDatabase

	// Ensure collections exist and attach JSON-Schema validators.
	// This runs first so indexes can be created on existing collections.
	logger.Info("ensuring collections and validators")
	if err := validators.EnsureAll(ctx, db); err != nil {
		logger.Error("failed to ensure validators", zap.Error(err))
		return err
	}

	// Ensure database indexes for query performance.
	logger.Info("ensuring database indexes")
	if err := indexes.EnsureAll(ctx, db); err != nil {
		logger.Error("failed to ensure indexes", zap.Error(err))
		return err
	}

	// Seed default data (pages, settings)
	logger.Info("seeding default data")
	if err := seeding.SeedAll(ctx, db, logger); err != nil {
		logger.Error("failed to seed default data", zap.Error(err))
		return err
	}

	logger.Info("database schema ensured successfully")
	return nil
}

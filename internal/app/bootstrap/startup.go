// internal/app/bootstrap/startup.go
package bootstrap

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"strings"
	"time"

	"github.com/dalemusser/stratasave/internal/app/resources"
	"github.com/dalemusser/stratasave/internal/app/system/tasks"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/config"
	"github.com/dalemusser/waffle/pantry/text"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Startup runs once after DB connections and schema/index setup are complete,
// but before the HTTP handler is built and requests are served.
//
// This is the place for one-time initialization that depends on having live
// database connections and fully loaded configuration. Unlike ConnectDB and
// EnsureSchema which focus on infrastructure, Startup is for application-level
// initialization.
//
// Common uses for Startup:
//   - Load shared templates from the resources directory
//   - Warm caches with frequently accessed data
//   - Initialize in-memory lookup tables
//   - Validate external service connectivity
//   - Set up background workers or scheduled tasks
//   - Perform health checks on dependencies
//
// Returning a non-nil error will abort startup and prevent the server from
// starting. Returning nil signals that initialization succeeded.
//
// The context will be cancelled if the process is asked to shut down while
// Startup is running; honor it in any long-running work.
func Startup(ctx context.Context, coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) error {
	resources.LoadSharedTemplates()

	// Note: Indexes are created in EnsureSchema via indexes.EnsureAll().
	// Store-level EnsureIndexes() calls are not needed here.

	// Seed admin user if configured
	if appCfg.SeedAdminEmail != "" {
		if err := ensureAdminUser(ctx, deps, appCfg.SeedAdminEmail, appCfg.SeedAdminName, logger); err != nil {
			logger.Error("failed to seed admin user", zap.Error(err))
			return err
		}
	}

	// Start background task runner
	startTaskRunner(deps.MongoDatabase, logger)

	return nil
}

// taskRunner is the global task runner instance, used for graceful shutdown.
var taskRunner *tasks.Runner

// startTaskRunner initializes and starts the background task runner.
func startTaskRunner(db *mongo.Database, logger *zap.Logger) {
	taskRunner = tasks.New(logger)

	// Register cleanup jobs
	taskRunner.Register(tasks.SessionCleanupJob(db, logger))
	taskRunner.Register(tasks.InvitationCleanupJob(db, logger))
	taskRunner.Register(tasks.PasswordResetCleanupJob(db, logger))
	taskRunner.Register(tasks.OAuthStateCleanupJob(db, logger))
	taskRunner.Register(tasks.EmailVerificationCleanupJob(db, logger))

	// Close sessions inactive for 30 minutes (checked every 5 minutes)
	taskRunner.Register(tasks.InactiveSessionCleanupJob(db, logger, 30*time.Minute))

	// Start running jobs
	taskRunner.Start()
}

// ensureAdminUser ensures an admin user exists with the given login_id.
// If a user exists with this login_id, ensure they have admin role.
// If no user exists, create a new admin user.
func ensureAdminUser(ctx context.Context, deps DBDeps, loginID string, name string, logger *zap.Logger) error {
	db := deps.MongoDatabase
	coll := db.Collection("users")

	loginID = strings.ToLower(strings.TrimSpace(loginID))
	if name == "" {
		name = "Admin"
	}

	// Check if user exists with this login_id
	var existingUser models.User
	err := coll.FindOne(ctx, bson.M{"login_id": loginID}).Decode(&existingUser)

	if err == nil {
		// User exists
		if existingUser.Role == "admin" {
			logger.Debug("admin user already configured", zap.String("login_id", loginID))
			return nil
		}

		// Promote to admin
		_, err = coll.UpdateByID(ctx, existingUser.ID, bson.M{
			"$set": bson.M{
				"role":       "admin",
				"updated_at": time.Now().UTC(),
			},
		})
		if err != nil {
			return err
		}
		logger.Info("promoted existing user to admin",
			zap.String("login_id", loginID),
			zap.String("user_id", existingUser.ID.Hex()),
			zap.String("previous_role", existingUser.Role))
		return nil
	}

	if err != mongo.ErrNoDocuments {
		return err
	}

	// Create new admin user
	now := time.Now().UTC()
	newUser := models.User{
		ID:         primitive.NewObjectID(),
		FullName:   name,
		FullNameCI: text.Fold(name),
		Email:      nil,
		LoginID:    &loginID,
		LoginIDCI:  ptrString(text.Fold(loginID)),
		AuthMethod: "trust", // Default to trust auth for seeded admin
		Role:       "admin",
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	_, err = coll.InsertOne(ctx, newUser)
	if err != nil {
		return err
	}

	logger.Info("created admin user",
		zap.String("login_id", loginID),
		zap.String("user_id", newUser.ID.Hex()))
	return nil
}

func ptrString(s string) *string {
	return &s
}

// internal/app/system/tasks/jobs.go
package tasks

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// SessionCleanupJob creates a job that removes expired sessions from the database.
func SessionCleanupJob(db *mongo.Database, logger *zap.Logger) Job {
	return Job{
		Name:     "session-cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context) error {
			coll := db.Collection("sessions")
			result, err := coll.DeleteMany(ctx, bson.M{
				"expires_at": bson.M{"$lt": time.Now()},
			})
			if err != nil {
				return err
			}
			if result.DeletedCount > 0 {
				logger.Info("cleaned up expired sessions",
					zap.Int64("deleted", result.DeletedCount))
			}
			return nil
		},
	}
}

// InvitationCleanupJob creates a job that removes expired and used invitations.
func InvitationCleanupJob(db *mongo.Database, logger *zap.Logger) Job {
	return Job{
		Name:     "invitation-cleanup",
		Interval: 6 * time.Hour,
		Run: func(ctx context.Context) error {
			coll := db.Collection("invitations")

			// Delete expired invitations (older than 30 days past expiry)
			cutoff := time.Now().Add(-30 * 24 * time.Hour)
			result, err := coll.DeleteMany(ctx, bson.M{
				"$or": []bson.M{
					{"expires_at": bson.M{"$lt": cutoff}},
					{"used_at": bson.M{"$lt": cutoff}},
				},
			})
			if err != nil {
				return err
			}
			if result.DeletedCount > 0 {
				logger.Info("cleaned up old invitations",
					zap.Int64("deleted", result.DeletedCount))
			}
			return nil
		},
	}
}

// PasswordResetCleanupJob creates a job that removes expired password reset tokens.
func PasswordResetCleanupJob(db *mongo.Database, logger *zap.Logger) Job {
	return Job{
		Name:     "password-reset-cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context) error {
			coll := db.Collection("password_resets")
			result, err := coll.DeleteMany(ctx, bson.M{
				"expires_at": bson.M{"$lt": time.Now()},
			})
			if err != nil {
				return err
			}
			if result.DeletedCount > 0 {
				logger.Info("cleaned up expired password reset tokens",
					zap.Int64("deleted", result.DeletedCount))
			}
			return nil
		},
	}
}

// OAuthStateCleanupJob creates a job that removes expired OAuth state tokens.
func OAuthStateCleanupJob(db *mongo.Database, logger *zap.Logger) Job {
	return Job{
		Name:     "oauth-state-cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context) error {
			coll := db.Collection("oauth_states")
			result, err := coll.DeleteMany(ctx, bson.M{
				"expires_at": bson.M{"$lt": time.Now()},
			})
			if err != nil {
				return err
			}
			if result.DeletedCount > 0 {
				logger.Info("cleaned up expired oauth states",
					zap.Int64("deleted", result.DeletedCount))
			}
			return nil
		},
	}
}

// EmailVerificationCleanupJob creates a job that removes expired email verification tokens.
func EmailVerificationCleanupJob(db *mongo.Database, logger *zap.Logger) Job {
	return Job{
		Name:     "email-verification-cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context) error {
			coll := db.Collection("email_verifications")
			result, err := coll.DeleteMany(ctx, bson.M{
				"expires_at": bson.M{"$lt": time.Now()},
			})
			if err != nil {
				return err
			}
			if result.DeletedCount > 0 {
				logger.Info("cleaned up expired email verification tokens",
					zap.Int64("deleted", result.DeletedCount))
			}
			return nil
		},
	}
}

// InactiveSessionCleanupJob creates a job that closes sessions inactive for longer than
// the specified threshold. This marks sessions as ended (with end_reason="inactive")
// rather than deleting them, preserving session history for auditing.
func InactiveSessionCleanupJob(db *mongo.Database, logger *zap.Logger, threshold time.Duration) Job {
	return Job{
		Name:     "inactive-session-cleanup",
		Interval: 5 * time.Minute,
		Run: func(ctx context.Context) error {
			coll := db.Collection("sessions")
			cutoff := time.Now().Add(-threshold)
			now := time.Now()

			result, err := coll.UpdateMany(ctx,
				bson.M{
					"logout_at":     nil,
					"last_activity": bson.M{"$lt": cutoff},
				},
				bson.M{
					"$set": bson.M{
						"logout_at":  now,
						"end_reason": "inactive",
						"updated_at": now,
					},
				},
			)
			if err != nil {
				return err
			}
			if result.ModifiedCount > 0 {
				logger.Info("closed inactive sessions",
					zap.Int64("count", result.ModifiedCount),
					zap.Duration("threshold", threshold))
			}
			return nil
		},
	}
}

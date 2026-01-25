// internal/app/system/indexes/indexes.go
package indexes

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

/*
EnsureAll is called at startup. Each ensure* function is idempotent.
We aggregate errors so any problem is visible and startup can fail fast.
*/
func EnsureAll(ctx context.Context, db *mongo.Database) error {
	var problems []string

	if err := ensureUsers(ctx, db); err != nil {
		problems = append(problems, "users: "+err.Error())
	}
	if err := ensurePages(ctx, db); err != nil {
		problems = append(problems, "pages: "+err.Error())
	}
	if err := ensureEmailVerifications(ctx, db); err != nil {
		problems = append(problems, "email_verifications: "+err.Error())
	}
	if err := ensureOAuthStates(ctx, db); err != nil {
		problems = append(problems, "oauth_states: "+err.Error())
	}
	if err := ensureSiteSettings(ctx, db); err != nil {
		problems = append(problems, "site_settings: "+err.Error())
	}
	if err := ensureAuditLogs(ctx, db); err != nil {
		problems = append(problems, "audit_logs: "+err.Error())
	}
	if err := ensureSessions(ctx, db); err != nil {
		problems = append(problems, "sessions: "+err.Error())
	}
	if err := ensureActivityEvents(ctx, db); err != nil {
		problems = append(problems, "activity_events: "+err.Error())
	}
	if err := ensureLoginRecords(ctx, db); err != nil {
		problems = append(problems, "login_records: "+err.Error())
	}
	if err := ensureRateLimits(ctx, db); err != nil {
		problems = append(problems, "rate_limits: "+err.Error())
	}
	if err := ensureFileFolders(ctx, db); err != nil {
		problems = append(problems, "file_folders: "+err.Error())
	}
	if err := ensureFiles(ctx, db); err != nil {
		problems = append(problems, "files: "+err.Error())
	}
	if err := ensureLedgerEntries(ctx, db); err != nil {
		problems = append(problems, "ledger_entries: "+err.Error())
	}
	if err := ensureAPIKeys(ctx, db); err != nil {
		problems = append(problems, "api_keys: "+err.Error())
	}
	if err := ensureJobs(ctx, db); err != nil {
		problems = append(problems, "jobs: "+err.Error())
	}
	if err := ensureDailyStats(ctx, db); err != nil {
		problems = append(problems, "daily_stats: "+err.Error())
	}
	if err := ensureSavedFilters(ctx, db); err != nil {
		problems = append(problems, "saved_filters: "+err.Error())
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

/* -------------------------------------------------------------------------- */
/* Core helper: reconcile a set of desired indexes for one collection         */
/* -------------------------------------------------------------------------- */

type existingIndex struct {
	Name   string `bson:"name"`
	Key    bson.D `bson:"key"`
	Unique *bool  `bson:"unique,omitempty"`
}

func keySig(keys bson.D) string {
	parts := make([]string, 0, len(keys))
	for _, kv := range keys {
		parts = append(parts, fmt.Sprintf("%s:%v", kv.Key, kv.Value))
	}
	return strings.Join(parts, ", ")
}

func sameBoolPtr(a, b *bool) bool {
	av := false
	bv := false
	if a != nil {
		av = *a
	}
	if b != nil {
		bv = *b
	}
	return av == bv
}

// Best-effort duplicate-detector (works cross-vendors)
func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == 11000 { // E11000 duplicate key error index
				return true
			}
		}
	}
	var ce mongo.CommandError
	if errors.As(err, &ce) && ce.Code == 11000 {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "E11000") || strings.Contains(strings.ToLower(s), "duplicate key")
}

// Mongo/DocDB sometimes returns IndexOptionsConflict when an index with the
// same keys already exists under a different name (or options differ).
func isOptionsConflictErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "IndexOptionsConflict")
}

func ensureIndexSet(ctx context.Context, coll *mongo.Collection, models []mongo.IndexModel) error {
	var errs []string

	for _, m := range models {
		var desiredName string
		var desiredUnique *bool
		if m.Options != nil {
			if m.Options.Name != nil {
				desiredName = *m.Options.Name
			}
			if m.Options.Unique != nil {
				desiredUnique = m.Options.Unique
			}
		}
		desiredSig := keySig(m.Keys.(bson.D))

		start := time.Now()
		zap.L().Info("ensuring index",
			zap.String("collection", coll.Name()),
			zap.String("name", desiredName),
			zap.String("keys", desiredSig),
			zap.Bool("unique", desiredUnique != nil && *desiredUnique))

		// 1) Load existing indexes
		existing := map[string]existingIndex{} // sig -> index
		cur, err := coll.Indexes().List(ctx)
		if err == nil {
			defer cur.Close(ctx)
			for cur.Next(ctx) {
				var idx existingIndex
				if err := cur.Decode(&idx); err != nil {
					zap.L().Warn("failed to decode existing index",
						zap.String("collection", coll.Name()),
						zap.Error(err))
					continue
				}
				existing[keySig(idx.Key)] = idx
			}
		}

		if ex, ok := existing[desiredSig]; ok {
			// Same key pattern exists already.
			if sameBoolPtr(desiredUnique, ex.Unique) {
				// Names aligned (or we don't care) → reuse
				zap.L().Info("reusing existing index",
					zap.String("collection", coll.Name()),
					zap.String("name", ex.Name),
					zap.String("keys", desiredSig),
					zap.Bool("unique", ex.Unique != nil && *ex.Unique),
					zap.String("took", time.Since(start).String()))
				continue
			}

			// Options mismatch (e.g., upgrading to unique). Drop & recreate.
			if _, err := coll.Indexes().DropOne(ctx, ex.Name); err != nil {
				zap.L().Warn("drop existing index failed",
					zap.String("collection", coll.Name()),
					zap.String("name", ex.Name),
					zap.String("keys", desiredSig),
					zap.Error(err))
				errs = append(errs, fmt.Sprintf("%s(%s): drop failed: %v", coll.Name(), desiredName, err))
				continue
			}
			if _, err := coll.Indexes().CreateOne(ctx, m); err != nil {
				if isDuplicateKeyErr(err) && desiredUnique != nil && *desiredUnique {
					errs = append(errs, fmt.Sprintf("%s(%s): cannot create unique index (duplicates present)", coll.Name(), desiredName))
				} else {
					errs = append(errs, fmt.Sprintf("%s(%s): %v", coll.Name(), desiredName, err))
				}
				continue
			}
			zap.L().Info("index dropped and recreated",
				zap.String("collection", coll.Name()),
				zap.String("name", desiredName),
				zap.String("keys", desiredSig),
				zap.Bool("unique", desiredUnique != nil && *desiredUnique),
				zap.String("took", time.Since(start).String()))
			continue
		}

		// 2) No existing index with the same keys: create it.
		if created, err := coll.Indexes().CreateOne(ctx, m); err != nil {
			if isOptionsConflictErr(err) {
				zap.L().Warn("index ensure failed (options conflict)",
					zap.String("collection", coll.Name()),
					zap.String("name", desiredName),
					zap.String("keys", desiredSig),
					zap.Error(err))
				errs = append(errs, fmt.Sprintf("%s(%s): %v", coll.Name(), desiredName, err))
				continue
			}

			zap.L().Warn("index ensure failed",
				zap.String("collection", coll.Name()),
				zap.String("name", desiredName),
				zap.String("keys", desiredSig),
				zap.Bool("unique", desiredUnique != nil && *desiredUnique),
				zap.String("took", time.Since(start).String()),
				zap.Error(err))
			errs = append(errs, fmt.Sprintf("%s(%s): %v", coll.Name(), desiredName, err))
			continue
		} else {
			zap.L().Info("index ensured",
				zap.String("collection", coll.Name()),
				zap.String("name", desiredName),
				zap.String("created_name", created),
				zap.String("keys", desiredSig),
				zap.Bool("unique", desiredUnique != nil && *desiredUnique),
				zap.String("took", time.Since(start).String()))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

/* -------------------------------------------------------------------------- */
/* Collection-specific index sets                                              */
/* -------------------------------------------------------------------------- */

func ensureUsers(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("users")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique login_id_ci + auth_method combination
		{
			Keys: bson.D{
				{Key: "login_id_ci", Value: 1},
				{Key: "auth_method", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetSparse(true).SetName("uniq_users_login_auth"),
		},

		// User list queries: role + status + name sort
		{
			Keys: bson.D{
				{Key: "role", Value: 1},
				{Key: "status", Value: 1},
				{Key: "full_name_ci", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_users_role_status_fullnameci_id"),
		},

		// Login ID search path
		{
			Keys: bson.D{
				{Key: "role", Value: 1},
				{Key: "status", Value: 1},
				{Key: "login_id_ci", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_users_role_status_loginidci_id"),
		},
	})
}

func ensurePages(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("pages")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique slug for each page (about, contact, terms-of-service, privacy-policy)
		{
			Keys: bson.D{
				{Key: "slug", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("uniq_pages_slug"),
		},
	})
}

func ensureEmailVerifications(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("email_verifications")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// TTL index for auto-cleanup of expired verifications
		{
			Keys: bson.D{
				{Key: "expires_at", Value: 1},
			},
			Options: options.Index().
				SetExpireAfterSeconds(0).
				SetName("idx_emailverify_expires_ttl"),
		},
		// Unique token for magic link verification (prevents token reuse)
		{
			Keys: bson.D{
				{Key: "token", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("uniq_emailverify_token"),
		},
		// Lookup by user_id (for code verification and cleanup)
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
			},
			Options: options.Index().
				SetName("idx_emailverify_user"),
		},
	})
}

func ensureOAuthStates(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("oauth_states")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique state token
		{
			Keys: bson.D{
				{Key: "state", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("uniq_oauth_state"),
		},
		// TTL index for auto-cleanup of expired states
		{
			Keys: bson.D{
				{Key: "expires_at", Value: 1},
			},
			Options: options.Index().
				SetExpireAfterSeconds(0).
				SetName("idx_oauth_expires_ttl"),
		},
	})
}

func ensureSiteSettings(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("site_settings")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique singleton - only one settings document
		{
			Keys: bson.D{
				{Key: "singleton", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("uniq_sitesettings_singleton"),
		},
	})
}

func ensureAuditLogs(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("audit_logs")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Time-based queries (most common)
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_audit_created"),
		},
		// Category + time queries
		{
			Keys: bson.D{
				{Key: "category", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_audit_category_created"),
		},
		// User-specific audit trail
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_audit_user_created"),
		},
		// Actor-specific audit trail
		{
			Keys: bson.D{
				{Key: "actor_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_audit_actor_created"),
		},
	})
}

func ensureSessions(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("sessions")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Lookup by token (unique)
		{
			Keys: bson.D{
				{Key: "token", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("idx_session_token"),
		},
		// Lookup by user
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
			},
			Options: options.Index().SetName("idx_session_user"),
		},
		// TTL index for automatic cleanup
		{
			Keys: bson.D{
				{Key: "expires_at", Value: 1},
			},
			Options: options.Index().SetExpireAfterSeconds(0).SetName("idx_session_ttl"),
		},
		// Active sessions query (who's online)
		{
			Keys: bson.D{
				{Key: "logout_at", Value: 1},
				{Key: "last_activity", Value: -1},
			},
			Options: options.Index().SetName("idx_session_active"),
		},
	})
}

func ensureActivityEvents(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("activity_events")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Activity by session (for session detail view)
		{
			Keys: bson.D{
				{Key: "session_id", Value: 1},
				{Key: "timestamp", Value: 1},
			},
			Options: options.Index().SetName("idx_activity_session"),
		},
		// Activity by user (for user activity history)
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_activity_user"),
		},
	})
}

func ensureLoginRecords(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("login_records")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Login history by user (for user login history)
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_logins_user_created"),
		},
		// Login history by time (for date range queries)
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_logins_created"),
		},
	})
}

func ensureRateLimits(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("rate_limits")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique login_id for fast lookups
		{
			Keys: bson.D{
				{Key: "login_id", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("idx_ratelimit_login_id"),
		},
		// TTL index on last_attempt - automatically clean up old records after 24 hours
		{
			Keys: bson.D{
				{Key: "last_attempt", Value: 1},
			},
			Options: options.Index().SetExpireAfterSeconds(86400).SetName("idx_ratelimit_ttl"),
		},
	})
}

func ensureFileFolders(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("file_folders")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique folder name within parent (prevents duplicate folder names)
		// This index also serves for listing folders by parent, sorted by name
		{
			Keys: bson.D{
				{Key: "parent_id", Value: 1},
				{Key: "name_ci", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uniq_folder_parent_name"),
		},
		// List folders by parent, sorted by date
		{
			Keys: bson.D{
				{Key: "parent_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_folder_parent_created"),
		},
	})
}

func ensureFiles(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("files")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique filename within folder (prevents duplicate filenames)
		// This index also serves for listing files by folder, sorted by name
		{
			Keys: bson.D{
				{Key: "folder_id", Value: 1},
				{Key: "name_ci", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uniq_file_folder_name"),
		},
		// List files by folder, sorted by date
		{
			Keys: bson.D{
				{Key: "folder_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_file_folder_created"),
		},
		// Filter files by content type
		{
			Keys: bson.D{
				{Key: "content_type", Value: 1},
			},
			Options: options.Index().SetName("idx_file_content_type"),
		},
	})
}

func ensureLedgerEntries(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("ledger_entries")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Time-based queries (most common)
		{
			Keys: bson.D{
				{Key: "started_at", Value: -1},
			},
			Options: options.Index().SetName("idx_ledger_started"),
		},
		// Unique request_id
		{
			Keys: bson.D{
				{Key: "request_id", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uniq_ledger_request_id"),
		},
		// Actor queries
		{
			Keys: bson.D{
				{Key: "actor_type", Value: 1},
				{Key: "actor_id", Value: 1},
				{Key: "started_at", Value: -1},
			},
			Options: options.Index().SetName("idx_ledger_actor"),
		},
		// Path queries
		{
			Keys: bson.D{
				{Key: "path", Value: 1},
				{Key: "started_at", Value: -1},
			},
			Options: options.Index().SetName("idx_ledger_path"),
		},
		// Status code queries
		{
			Keys: bson.D{
				{Key: "status_code", Value: 1},
				{Key: "started_at", Value: -1},
			},
			Options: options.Index().SetName("idx_ledger_status"),
		},
		// Error queries
		{
			Keys: bson.D{
				{Key: "error_class", Value: 1},
				{Key: "started_at", Value: -1},
			},
			Options: options.Index().SetSparse(true).SetName("idx_ledger_error_class"),
		},
	})
}

func ensureAPIKeys(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("api_keys")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique name per API key
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uniq_apikey_name"),
		},
		// Lookup by key prefix for validation
		{
			Keys: bson.D{
				{Key: "key_prefix", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("idx_apikey_prefix_status"),
		},
		// List by status and creation date
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_apikey_status_created"),
		},
		// Created by (for audit)
		{
			Keys: bson.D{
				{Key: "created_by", Value: 1},
			},
			Options: options.Index().SetName("idx_apikey_created_by"),
		},
	})
}

func ensureJobs(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("jobs")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Claim next job: queue + status + scheduled_at + priority
		{
			Keys: bson.D{
				{Key: "queue_name", Value: 1},
				{Key: "status", Value: 1},
				{Key: "priority", Value: -1},
				{Key: "scheduled_at", Value: 1},
			},
			Options: options.Index().SetName("idx_job_claim"),
		},
		// List by status
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_job_status_created"),
		},
		// Cleanup stale running jobs
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "started_at", Value: 1},
			},
			Options: options.Index().SetName("idx_job_status_started"),
		},
		// Job type queries
		{
			Keys: bson.D{
				{Key: "job_type", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("idx_job_type_status"),
		},
		// Cleanup completed jobs
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "completed_at", Value: 1},
			},
			Options: options.Index().SetName("idx_job_status_completed"),
		},
	})
}

func ensureDailyStats(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("daily_stats")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique date + stat_type combination
		{
			Keys: bson.D{
				{Key: "date", Value: 1},
				{Key: "stat_type", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uniq_stats_date_type"),
		},
		// Range queries by stat type
		{
			Keys: bson.D{
				{Key: "stat_type", Value: 1},
				{Key: "date", Value: 1},
			},
			Options: options.Index().SetName("idx_stats_type_date"),
		},
	})
}

func ensureSavedFilters(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("saved_filters")
	return ensureIndexSet(ctx, c, []mongo.IndexModel{
		// Unique name per user/feature
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "feature", Value: 1},
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uniq_filter_user_feature_name"),
		},
		// List filters for user/feature
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "feature", Value: 1},
				{Key: "is_default", Value: -1},
			},
			Options: options.Index().SetName("idx_filter_user_feature"),
		},
	})
}

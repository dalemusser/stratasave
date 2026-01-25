// internal/app/store/audit/store.go
package audit

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Event categories
const (
	CategoryAuth  = "auth"
	CategoryAdmin = "admin"
)

// Auth event types
const (
	EventLoginSuccess             = "login_success"
	EventLoginFailedUserNotFound  = "login_failed_user_not_found"
	EventLoginFailedWrongPassword = "login_failed_wrong_password"
	EventLoginFailedUserDisabled  = "login_failed_user_disabled"
	EventLoginRateLimited         = "login_rate_limited"
	EventLoginLockedOut           = "login_locked_out"
	EventLogout                   = "logout"
	EventPasswordChanged          = "password_changed"
	EventVerificationCodeSent     = "verification_code_sent"
	EventVerificationCodeResent   = "verification_code_resent"
	EventVerificationCodeFailed   = "verification_code_failed"
	EventMagicLinkUsed            = "magic_link_used"
)

// Admin event types
const (
	EventUserCreated     = "user_created"
	EventUserUpdated     = "user_updated"
	EventUserDisabled    = "user_disabled"
	EventUserEnabled     = "user_enabled"
	EventUserDeleted     = "user_deleted"
	EventSettingsUpdated = "settings_updated"
	EventPageUpdated     = "page_updated"
)

// Event represents an audit event.
type Event struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	CreatedAt time.Time          `bson:"created_at"`

	// Event classification
	Category  string `bson:"category"`
	EventType string `bson:"event_type"`

	// Who
	UserID  *primitive.ObjectID `bson:"user_id,omitempty"`  // affected user
	ActorID *primitive.ObjectID `bson:"actor_id,omitempty"` // who performed action (for admin actions)

	// Context
	IP        string `bson:"ip"`
	UserAgent string `bson:"user_agent,omitempty"`

	// Outcome
	Success       bool   `bson:"success"`
	FailureReason string `bson:"failure_reason,omitempty"`

	// Additional details (varies by event type)
	Details map[string]string `bson:"details,omitempty"`
}

// QueryFilter defines filters for querying audit events.
type QueryFilter struct {
	UserID    *primitive.ObjectID
	ActorID   *primitive.ObjectID
	Category  string
	EventType string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int64
	Offset    int64
}

// Store manages audit event records.
type Store struct {
	c *mongo.Collection
}

// New creates a new audit Store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("audit_logs")}
}

// EnsureIndexes creates indexes for efficient querying.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// Query by user (affected user)
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_audit_user"),
		},
		// Query by actor (who performed action)
		{
			Keys:    bson.D{{Key: "actor_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_audit_actor"),
		},
		// Query by category
		{
			Keys:    bson.D{{Key: "category", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_audit_category"),
		},
		// Query by event type
		{
			Keys:    bson.D{{Key: "event_type", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_audit_event_type"),
		},
		// Time-based queries
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_audit_created"),
		},
	}
	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// Log records an audit event.
func (s *Store) Log(ctx context.Context, event Event) error {
	if event.ID.IsZero() {
		event.ID = primitive.NewObjectID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	_, err := s.c.InsertOne(ctx, event)
	return err
}

// Query retrieves audit events matching the given filter.
func (s *Store) Query(ctx context.Context, filter QueryFilter) ([]Event, error) {
	query := bson.M{}

	if filter.UserID != nil {
		query["user_id"] = filter.UserID
	}
	if filter.ActorID != nil {
		query["actor_id"] = filter.ActorID
	}
	if filter.Category != "" {
		query["category"] = filter.Category
	}
	if filter.EventType != "" {
		query["event_type"] = filter.EventType
	}

	// Time range
	if filter.StartTime != nil || filter.EndTime != nil {
		timeQuery := bson.M{}
		if filter.StartTime != nil {
			timeQuery["$gte"] = *filter.StartTime
		}
		if filter.EndTime != nil {
			timeQuery["$lte"] = *filter.EndTime
		}
		query["created_at"] = timeQuery
	}

	// Set defaults
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(filter.Offset)

	cursor, err := s.c.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []Event
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// CountByFilter returns the count of events matching the filter.
func (s *Store) CountByFilter(ctx context.Context, filter QueryFilter) (int64, error) {
	query := bson.M{}

	if filter.UserID != nil {
		query["user_id"] = filter.UserID
	}
	if filter.ActorID != nil {
		query["actor_id"] = filter.ActorID
	}
	if filter.Category != "" {
		query["category"] = filter.Category
	}
	if filter.EventType != "" {
		query["event_type"] = filter.EventType
	}

	if filter.StartTime != nil || filter.EndTime != nil {
		timeQuery := bson.M{}
		if filter.StartTime != nil {
			timeQuery["$gte"] = *filter.StartTime
		}
		if filter.EndTime != nil {
			timeQuery["$lte"] = *filter.EndTime
		}
		query["created_at"] = timeQuery
	}

	return s.c.CountDocuments(ctx, query)
}

// GetByUser retrieves recent audit events for a specific user.
func (s *Store) GetByUser(ctx context.Context, userID primitive.ObjectID, limit int64) ([]Event, error) {
	return s.Query(ctx, QueryFilter{
		UserID: &userID,
		Limit:  limit,
	})
}

// GetRecent retrieves the most recent audit events.
func (s *Store) GetRecent(ctx context.Context, limit int64) ([]Event, error) {
	return s.Query(ctx, QueryFilter{
		Limit: limit,
	})
}

// GetFailedLogins retrieves recent failed login attempts.
func (s *Store) GetFailedLogins(ctx context.Context, since time.Time, limit int64) ([]Event, error) {
	query := bson.M{
		"category": CategoryAuth,
		"success":  false,
		"event_type": bson.M{
			"$in": []string{
				EventLoginFailedUserNotFound,
				EventLoginFailedWrongPassword,
				EventLoginFailedUserDisabled,
			},
		},
		"created_at": bson.M{"$gte": since},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)

	cursor, err := s.c.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []Event
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

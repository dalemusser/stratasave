// internal/app/store/activity/store.go
package activity

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

// Event types for activity tracking.
// Note: Login/logout events are NOT tracked here - they are captured in the sessions store
// (login_at, logout_at fields) to avoid redundancy. The Activity History display creates
// synthetic login/logout events from session data.
const (
	EventPageView = "page_view" // User viewed a page
)

// Event represents a user activity event.
type Event struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	SessionID primitive.ObjectID `bson:"session_id"`
	Timestamp time.Time          `bson:"timestamp"`

	// What happened
	EventType string `bson:"event_type"`

	// Context (varies by event type)
	PagePath string         `bson:"page_path,omitempty"`
	Details  map[string]any `bson:"details,omitempty"`
}

// Store manages activity events.
type Store struct {
	c *mongo.Collection
}

// New creates a new activity Store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("activity_events")}
}

// EnsureIndexes creates necessary indexes for efficient querying.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// Activity by session (for session detail view)
		{
			Keys:    bson.D{{Key: "session_id", Value: 1}, {Key: "timestamp", Value: 1}},
			Options: options.Index().SetName("idx_activity_session"),
		},
		// Activity by user (for user activity history)
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("idx_activity_user"),
		},
	}
	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// Create records a new activity event.
func (s *Store) Create(ctx context.Context, event Event) error {
	if event.ID.IsZero() {
		event.ID = primitive.NewObjectID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	_, err := s.c.InsertOne(ctx, event)
	return err
}

// RecordPageView records when a user views a page.
func (s *Store) RecordPageView(ctx context.Context, userID, sessionID primitive.ObjectID, pagePath string) error {
	event := Event{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		SessionID: sessionID,
		Timestamp: time.Now().UTC(),
		EventType: EventPageView,
		PagePath:  pagePath,
	}
	_, err := s.c.InsertOne(ctx, event)
	return err
}


// GetBySession retrieves all events for a session.
func (s *Store) GetBySession(ctx context.Context, sessionID primitive.ObjectID) ([]Event, error) {
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cur, err := s.c.Find(ctx, bson.M{"session_id": sessionID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []Event
	if err := cur.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// GetByUser retrieves recent events for a user.
func (s *Store) GetByUser(ctx context.Context, userID primitive.ObjectID, limit int64) ([]Event, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(limit)

	cur, err := s.c.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []Event
	if err := cur.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// GetByUserInTimeRange retrieves events for a user within a time range.
func (s *Store) GetByUserInTimeRange(ctx context.Context, userID primitive.ObjectID, start, end time.Time) ([]Event, error) {
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})

	filter := bson.M{
		"user_id": userID,
		"timestamp": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	cur, err := s.c.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []Event
	if err := cur.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// CountByUserInTimeRange counts events for a user in a time range.
func (s *Store) CountByUserInTimeRange(ctx context.Context, userID primitive.ObjectID, eventType string, start, end time.Time) (int64, error) {
	filter := bson.M{
		"user_id":    userID,
		"event_type": eventType,
		"timestamp": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}
	return s.c.CountDocuments(ctx, filter)
}

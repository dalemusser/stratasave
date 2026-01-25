// internal/app/store/sessions/store.go
package sessions

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

// Session end reasons
const (
	EndReasonLogout   = "logout"   // User explicitly logged out
	EndReasonExpired  = "expired"  // Session expired via TTL
	EndReasonInactive = "inactive" // Closed due to inactivity
)

// Session represents a stored session in the database.
// This is used for server-side session storage and activity tracking.
type Session struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Token     string             `bson:"token"`   // Unique 32-byte random token
	UserID    primitive.ObjectID `bson:"user_id"`
	IPAddress string             `bson:"ip_address,omitempty"`
	UserAgent string             `bson:"user_agent,omitempty"`
	Data      map[string]any     `bson:"data,omitempty"`

	// Activity tracking
	CurrentPage      string     `bson:"current_page,omitempty"`       // Current page user is viewing
	LoginAt          time.Time  `bson:"login_at"`                     // When session started
	LogoutAt         *time.Time `bson:"logout_at,omitempty"`          // When session ended (nil if active)
	LastActivity     time.Time  `bson:"last_activity"`                // Last heartbeat (tab open)
	LastUserActivity time.Time  `bson:"last_user_activity,omitempty"` // Last real user interaction (clicks, keys)
	EndReason        string     `bson:"end_reason,omitempty"`         // "logout", "expired", "inactive"
	DurationSecs     int64      `bson:"duration_secs,omitempty"`      // Computed on close

	// TTL expiration
	ExpiresAt time.Time `bson:"expires_at"`

	// Timestamps
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// Store manages session records in MongoDB.
// Note: Strata primarily uses cookie-based sessions via gorilla/sessions.
// This store is provided for scenarios requiring server-side session storage.
type Store struct {
	c *mongo.Collection
}

// New creates a new session Store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("sessions")}
}

// EnsureIndexes creates indexes for efficient querying and TTL expiration.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// Lookup by token (unique)
		{
			Keys:    bson.D{{Key: "token", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_session_token"),
		},
		// Lookup by user
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetName("idx_session_user"),
		},
		// TTL index for automatic cleanup
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName("idx_session_ttl"),
		},
		// Active sessions query (who's online)
		{
			Keys:    bson.D{{Key: "logout_at", Value: 1}, {Key: "last_activity", Value: -1}},
			Options: options.Index().SetName("idx_session_active"),
		},
	}
	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// Create creates a new session.
func (s *Store) Create(ctx context.Context, session Session) error {
	if session.ID.IsZero() {
		session.ID = primitive.NewObjectID()
	}
	now := time.Now()
	session.CreatedAt = now
	session.UpdatedAt = now
	if session.LoginAt.IsZero() {
		session.LoginAt = now
	}
	if session.LastActivity.IsZero() {
		session.LastActivity = now
	}
	if session.LastUserActivity.IsZero() {
		session.LastUserActivity = now
	}
	_, err := s.c.InsertOne(ctx, session)
	return err
}

// GetByToken retrieves an active session by token.
// Returns nil if the session has been logged out or expired.
func (s *Store) GetByToken(ctx context.Context, token string) (*Session, error) {
	var session Session
	err := s.c.FindOne(ctx, bson.M{
		"token":      token,
		"logout_at":  nil,
		"expires_at": bson.M{"$gt": time.Now()},
	}).Decode(&session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// Delete removes a session by token.
func (s *Store) Delete(ctx context.Context, token string) error {
	_, err := s.c.DeleteOne(ctx, bson.M{"token": token})
	return err
}

// DeleteByUser removes all sessions for a user.
func (s *Store) DeleteByUser(ctx context.Context, userID primitive.ObjectID) error {
	_, err := s.c.DeleteMany(ctx, bson.M{"user_id": userID})
	return err
}

// DeleteByID removes a session by ID.
func (s *Store) DeleteByID(ctx context.Context, id primitive.ObjectID) error {
	_, err := s.c.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// DeleteByUserExcept removes all sessions for a user except the specified token.
func (s *Store) DeleteByUserExcept(ctx context.Context, userID primitive.ObjectID, exceptToken string) error {
	_, err := s.c.DeleteMany(ctx, bson.M{
		"user_id": userID,
		"token":   bson.M{"$ne": exceptToken},
	})
	return err
}

// ListByUser retrieves all active sessions for a user.
func (s *Store) ListByUser(ctx context.Context, userID primitive.ObjectID) ([]Session, error) {
	cursor, err := s.c.Find(ctx, bson.M{
		"user_id":    userID,
		"expires_at": bson.M{"$gt": time.Now()},
	}, options.Find().SetSort(bson.D{{Key: "last_activity", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sessions []Session
	if err := cursor.All(ctx, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// UpdateActivity updates the last activity time and optionally the IP and user agent.
func (s *Store) UpdateActivity(ctx context.Context, token string, ip string, userAgent string) error {
	update := bson.M{
		"$set": bson.M{
			"last_activity": time.Now(),
			"updated_at":    time.Now(),
		},
	}

	if ip != "" {
		update["$set"].(bson.M)["ip_address"] = ip
	}
	if userAgent != "" {
		update["$set"].(bson.M)["user_agent"] = userAgent
	}

	_, err := s.c.UpdateOne(ctx, bson.M{"token": token}, update)
	return err
}

// UpdateUserActivity updates the last_user_activity timestamp for a session.
// This is called only when the user has actually interacted (clicks, keystrokes, scrolling).
// Unlike LastActivity (updated by every heartbeat), this tracks real user engagement.
func (s *Store) UpdateUserActivity(ctx context.Context, token string) error {
	now := time.Now()
	_, err := s.c.UpdateOne(ctx,
		bson.M{"token": token, "logout_at": nil},
		bson.M{"$set": bson.M{
			"last_user_activity": now,
			"updated_at":         now,
		}},
	)
	return err
}

// GetByID retrieves a session by ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*Session, error) {
	var session Session
	err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// Close closes a session with a reason and computes the duration.
// This marks the session as ended but does not delete it (for audit purposes).
func (s *Store) Close(ctx context.Context, token string, reason string) error {
	// First get the session to compute duration
	var session Session
	err := s.c.FindOne(ctx, bson.M{"token": token}).Decode(&session)
	if err != nil {
		return err
	}

	now := time.Now()
	duration := int64(now.Sub(session.LoginAt).Seconds())

	_, err = s.c.UpdateOne(ctx, bson.M{"token": token}, bson.M{
		"$set": bson.M{
			"logout_at":     now,
			"end_reason":    reason,
			"duration_secs": duration,
			"updated_at":    now,
		},
	})
	return err
}

// CloseByUser closes all sessions for a user with the given reason.
func (s *Store) CloseByUser(ctx context.Context, userID primitive.ObjectID, reason string) error {
	now := time.Now()
	_, err := s.c.UpdateMany(ctx,
		bson.M{
			"user_id":   userID,
			"logout_at": nil,
		},
		bson.M{
			"$set": bson.M{
				"logout_at":  now,
				"end_reason": reason,
				"updated_at": now,
			},
		},
	)
	return err
}

// CloseByUserExcept closes all sessions for a user except the specified token.
func (s *Store) CloseByUserExcept(ctx context.Context, userID primitive.ObjectID, exceptToken string, reason string) error {
	now := time.Now()
	_, err := s.c.UpdateMany(ctx,
		bson.M{
			"user_id":   userID,
			"token":     bson.M{"$ne": exceptToken},
			"logout_at": nil,
		},
		bson.M{
			"$set": bson.M{
				"logout_at":  now,
				"end_reason": reason,
				"updated_at": now,
			},
		},
	)
	return err
}

// UpdateResult contains the result of an UpdateCurrentPage operation.
type UpdateResult struct {
	Updated      bool   // Whether the session was updated
	PreviousPage string // The previous current_page value (before update)
}

// UpdateCurrentPage updates the current page and last activity time for a session.
// Only updates sessions that are not already closed (logout_at is nil).
// Returns UpdateResult with whether session was updated and the previous page value.
func (s *Store) UpdateCurrentPage(ctx context.Context, token string, page string) (UpdateResult, error) {
	now := time.Now()
	update := bson.M{
		"last_activity": now,
		"updated_at":    now,
	}
	if page != "" {
		update["current_page"] = page
	}

	// Use FindOneAndUpdate to get the previous state
	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.Before) // Return document BEFORE update

	var oldSession struct {
		CurrentPage string `bson:"current_page"`
	}
	err := s.c.FindOneAndUpdate(ctx,
		bson.M{
			"token":     token,
			"logout_at": nil, // Only update if session is still active
		},
		bson.M{"$set": update},
		opts,
	).Decode(&oldSession)

	if err == mongo.ErrNoDocuments {
		return UpdateResult{Updated: false}, nil
	}
	if err != nil {
		return UpdateResult{}, err
	}

	return UpdateResult{
		Updated:      true,
		PreviousPage: oldSession.CurrentPage,
	}, nil
}

// CloseInactiveSessions closes sessions that haven't had activity within the threshold.
// Returns the number of sessions closed.
func (s *Store) CloseInactiveSessions(ctx context.Context, threshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-threshold)
	now := time.Now()

	result, err := s.c.UpdateMany(ctx,
		bson.M{
			"logout_at":     nil,
			"last_activity": bson.M{"$lt": cutoff},
		},
		bson.M{
			"$set": bson.M{
				"logout_at":  now,
				"end_reason": EndReasonInactive,
				"updated_at": now,
			},
		},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

// GetActiveSessions retrieves all currently active (not logged out) sessions.
func (s *Store) GetActiveSessions(ctx context.Context, limit int64) ([]Session, error) {
	opts := options.Find().SetSort(bson.D{{Key: "last_activity", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}

	cursor, err := s.c.Find(ctx, bson.M{
		"logout_at":  nil,
		"expires_at": bson.M{"$gt": time.Now()},
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sessions []Session
	if err := cursor.All(ctx, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// GetActiveByUser retrieves all active (not logged out) sessions for a user.
func (s *Store) GetActiveByUser(ctx context.Context, userID primitive.ObjectID) ([]Session, error) {
	cursor, err := s.c.Find(ctx, bson.M{
		"user_id":    userID,
		"logout_at":  nil,
		"expires_at": bson.M{"$gt": time.Now()},
	}, options.Find().SetSort(bson.D{{Key: "last_activity", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sessions []Session
	if err := cursor.All(ctx, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// CountActive counts currently active sessions (not logged out and not expired).
func (s *Store) CountActive(ctx context.Context) (int64, error) {
	return s.c.CountDocuments(ctx, bson.M{
		"logout_at":  nil,
		"expires_at": bson.M{"$gt": time.Now()},
	})
}

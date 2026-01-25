// internal/app/store/ratelimit/store.go
package ratelimit

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Attempt tracks failed login attempts for a specific login_id.
type Attempt struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	LoginID      string             `bson:"login_id"`      // Normalized login identifier (lowercase)
	AttemptCount int                `bson:"attempt_count"` // Failed attempts in current window
	WindowStart  time.Time          `bson:"window_start"`  // When the current counting window started
	LockedUntil  *time.Time         `bson:"locked_until"`  // Lockout expiry time (nil if not locked)
	LastAttempt  time.Time          `bson:"last_attempt"`  // Most recent attempt (for TTL cleanup)
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
}

// Store manages rate limit tracking for login attempts.
type Store struct {
	c               *mongo.Collection
	maxAttempts     int
	windowDuration  time.Duration
	lockoutDuration time.Duration
}

// New creates a new rate limit Store with the given configuration.
func New(db *mongo.Database, maxAttempts int, window, lockout time.Duration) *Store {
	return &Store{
		c:               db.Collection("rate_limits"),
		maxAttempts:     maxAttempts,
		windowDuration:  window,
		lockoutDuration: lockout,
	}
}

// EnsureIndexes creates necessary indexes for efficient querying.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// Unique index on login_id for fast lookups
		{
			Keys:    bson.D{{Key: "login_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_ratelimit_login_id"),
		},
		// TTL index on last_attempt - automatically clean up old records after 24 hours
		{
			Keys:    bson.D{{Key: "last_attempt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(86400).SetName("idx_ratelimit_ttl"),
		},
	}
	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// normalizeLoginID converts login_id to lowercase for consistent lookups.
func normalizeLoginID(loginID string) string {
	return strings.ToLower(strings.TrimSpace(loginID))
}

// CheckAllowed checks if the given login_id is allowed to attempt login.
// Returns:
//   - allowed: true if login attempt should be processed
//   - remaining: number of attempts remaining before lockout (-1 if locked)
//   - lockedUntil: when the lockout expires (nil if not locked)
func (s *Store) CheckAllowed(ctx context.Context, loginID string) (allowed bool, remaining int, lockedUntil *time.Time) {
	loginID = normalizeLoginID(loginID)
	now := time.Now()

	var attempt Attempt
	err := s.c.FindOne(ctx, bson.M{"login_id": loginID}).Decode(&attempt)
	if err == mongo.ErrNoDocuments {
		// No record exists - allowed with full attempts remaining
		return true, s.maxAttempts, nil
	}
	if err != nil {
		// On error, allow the attempt (fail open for availability)
		return true, s.maxAttempts, nil
	}

	// Check if currently locked out
	if attempt.LockedUntil != nil && now.Before(*attempt.LockedUntil) {
		return false, -1, attempt.LockedUntil
	}

	// Check if window has expired (reset counter)
	if now.After(attempt.WindowStart.Add(s.windowDuration)) {
		return true, s.maxAttempts, nil
	}

	// Within window - check remaining attempts
	remaining = s.maxAttempts - attempt.AttemptCount
	if remaining <= 0 {
		// Should be locked but lockout wasn't set properly - treat as locked
		return false, 0, nil
	}

	return true, remaining, nil
}

// RecordFailure records a failed login attempt for the given login_id.
// Returns:
//   - lockedOut: true if this failure triggered a lockout
//   - lockedUntil: when the lockout expires (nil if not locked)
func (s *Store) RecordFailure(ctx context.Context, loginID string) (lockedOut bool, lockedUntil *time.Time) {
	loginID = normalizeLoginID(loginID)
	now := time.Now()

	// Try to find existing record
	var attempt Attempt
	err := s.c.FindOne(ctx, bson.M{"login_id": loginID}).Decode(&attempt)

	if err == mongo.ErrNoDocuments {
		// First failure - create new record
		attempt = Attempt{
			ID:           primitive.NewObjectID(),
			LoginID:      loginID,
			AttemptCount: 1,
			WindowStart:  now,
			LastAttempt:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		// Check if this single attempt triggers lockout (shouldn't with default settings)
		if attempt.AttemptCount >= s.maxAttempts {
			lockoutTime := now.Add(s.lockoutDuration)
			attempt.LockedUntil = &lockoutTime
			lockedOut = true
			lockedUntil = &lockoutTime
		}

		_, _ = s.c.InsertOne(ctx, attempt)
		return lockedOut, lockedUntil
	}

	if err != nil {
		// On error, don't lock (fail open)
		return false, nil
	}

	// Check if window has expired - reset counter
	if now.After(attempt.WindowStart.Add(s.windowDuration)) {
		attempt.AttemptCount = 1
		attempt.WindowStart = now
		attempt.LockedUntil = nil
	} else {
		attempt.AttemptCount++
	}

	attempt.LastAttempt = now
	attempt.UpdatedAt = now

	// Check if we've exceeded the limit
	if attempt.AttemptCount >= s.maxAttempts {
		lockoutTime := now.Add(s.lockoutDuration)
		attempt.LockedUntil = &lockoutTime
		lockedOut = true
		lockedUntil = &lockoutTime
	}

	// Update the record
	_, _ = s.c.UpdateOne(ctx,
		bson.M{"_id": attempt.ID},
		bson.M{"$set": bson.M{
			"attempt_count": attempt.AttemptCount,
			"window_start":  attempt.WindowStart,
			"locked_until":  attempt.LockedUntil,
			"last_attempt":  attempt.LastAttempt,
			"updated_at":    attempt.UpdatedAt,
		}},
	)

	return lockedOut, lockedUntil
}

// ClearOnSuccess removes the rate limit record for the given login_id.
// Called after a successful login to reset the counter.
func (s *Store) ClearOnSuccess(ctx context.Context, loginID string) error {
	loginID = normalizeLoginID(loginID)
	_, err := s.c.DeleteOne(ctx, bson.M{"login_id": loginID})
	return err
}

// GetAttempt returns the current attempt record for a login_id (for debugging/admin).
func (s *Store) GetAttempt(ctx context.Context, loginID string) (*Attempt, error) {
	loginID = normalizeLoginID(loginID)
	var attempt Attempt
	err := s.c.FindOne(ctx, bson.M{"login_id": loginID}).Decode(&attempt)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// internal/app/store/logins/loginstore.go
package loginstore

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	c *mongo.Collection
}

func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("login_records")}
}

// EnsureIndexes creates indexes for efficient querying.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// Per-user recent logins (latest-first)
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_logins_user_created"),
		},
		// Site-wide recent logins (latest-first)
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_logins_created"),
		},
	}
	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// Create inserts a LoginRecord. If CreatedAt is zero, it's set to time.Now().UTC().
func (s *Store) Create(ctx context.Context, rec models.LoginRecord) error {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	_, err := s.c.InsertOne(ctx, rec)
	return err
}

// CreateFrom builds a LoginRecord from the HTTP request and inserts it.
// It extracts client IP (X-Forwarded-For → X-Real-IP → RemoteAddr) and user agent.
func (s *Store) CreateFrom(ctx context.Context, r *http.Request, userID primitive.ObjectID, provider string) error {
	rec := models.LoginRecord{
		UserID:    userID.Hex(),
		CreatedAt: time.Now().UTC(),
		IP:        clientIP(r),
		Provider:  provider,
	}
	_, err := s.c.InsertOne(ctx, rec)
	return err
}

// GetByUser retrieves recent login records for a user.
func (s *Store) GetByUser(ctx context.Context, userID primitive.ObjectID, limit int64) ([]models.LoginRecord, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)

	cur, err := s.c.Find(ctx, bson.M{"user_id": userID.Hex()}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var records []models.LoginRecord
	if err := cur.All(ctx, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// GetByTimeRange retrieves login records within a time range.
func (s *Store) GetByTimeRange(ctx context.Context, start, end time.Time) ([]models.LoginRecord, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	filter := bson.M{
		"created_at": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	cur, err := s.c.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var records []models.LoginRecord
	if err := cur.All(ctx, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func clientIP(r *http.Request) string {
	// Respect common proxy headers first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// XFF may contain a list; first is original client
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	// Fallback: parse RemoteAddr "ip:port"
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

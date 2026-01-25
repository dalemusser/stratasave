// internal/app/store/ledger/ledgerstore.go
package ledgerstore

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Entry represents a single request log in the ledger.
type Entry struct {
	ID primitive.ObjectID `bson:"_id"`

	// Request identification
	RequestID       string `bson:"request_id"`                  // Generated UUID
	TraceID         string `bson:"trace_id,omitempty"`          // For distributed tracing
	ClientRequestID string `bson:"client_request_id,omitempty"` // From X-Request-ID header

	// HTTP request metadata
	Method   string            `bson:"method"`
	Path     string            `bson:"path"`
	Query    string            `bson:"query,omitempty"`
	Headers  map[string]string `bson:"headers,omitempty"` // Redacted sensitive headers
	RemoteIP string            `bson:"remote_ip"`

	// Actor identification
	ActorType string `bson:"actor_type"`           // "api_key", "session", "anonymous"
	ActorID   string `bson:"actor_id,omitempty"`   // API key ID or user ID
	ActorName string `bson:"actor_name,omitempty"` // Display name

	// Request body handling
	RequestBodySize    int64  `bson:"request_body_size"`
	RequestBodyHash    string `bson:"request_body_hash,omitempty"`    // SHA256 first 8 chars
	RequestBodyPreview string `bson:"request_body_preview,omitempty"` // First 500 chars
	RequestContentType string `bson:"request_content_type,omitempty"`

	// Response metadata
	StatusCode   int    `bson:"status_code"`
	ResponseSize int64  `bson:"response_size"`
	ErrorClass   string `bson:"error_class,omitempty"`   // "validation", "auth", "internal"
	ErrorMessage string `bson:"error_message,omitempty"` // Safe error message

	// Timing breakdown
	Timing TimingInfo `bson:"timing"`

	// Timestamps
	StartedAt   time.Time `bson:"started_at"`
	CompletedAt time.Time `bson:"completed_at"`

	// App-specific metadata (flexible)
	Metadata map[string]any `bson:"metadata,omitempty"`
}

// TimingInfo holds timing breakdown for a request.
type TimingInfo struct {
	DecodeMs   float64 `bson:"decode_ms,omitempty"`
	ValidateMs float64 `bson:"validate_ms,omitempty"`
	DBQueryMs  float64 `bson:"db_query_ms,omitempty"`
	EncodeMs   float64 `bson:"encode_ms,omitempty"`
	TotalMs    float64 `bson:"total_ms"`
}

// Store provides ledger entry persistence.
type Store struct {
	c *mongo.Collection
}

// New creates a new ledger store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("ledger_entries")}
}

// Create inserts a new ledger entry.
func (s *Store) Create(ctx context.Context, entry Entry) error {
	if entry.ID.IsZero() {
		entry.ID = primitive.NewObjectID()
	}
	_, err := s.c.InsertOne(ctx, entry)
	return err
}

// GetByID retrieves a ledger entry by ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*Entry, error) {
	var entry Entry
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetByRequestID retrieves a ledger entry by request ID.
func (s *Store) GetByRequestID(ctx context.Context, requestID string) (*Entry, error) {
	var entry Entry
	if err := s.c.FindOne(ctx, bson.M{"request_id": requestID}).Decode(&entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ListFilter specifies criteria for listing ledger entries.
type ListFilter struct {
	// Time range
	StartTime *time.Time
	EndTime   *time.Time

	// Actor filters
	ActorType string
	ActorID   string

	// Request filters
	Method     string
	PathPrefix string
	Path       string

	// Response filters
	StatusCodeMin *int
	StatusCodeMax *int
	ErrorClass    string

	// Search
	Search string // Searches request_id, path, actor_name
}

// ListResult contains a page of ledger entries with pagination info.
type ListResult struct {
	Entries    []Entry
	TotalCount int64
	Page       int
	PageSize   int
	TotalPages int
}

// List returns ledger entries matching the filter with pagination.
func (s *Store) List(ctx context.Context, filter ListFilter, page, pageSize int) (ListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := s.buildQuery(filter)

	// Count total
	total, err := s.c.CountDocuments(ctx, query)
	if err != nil {
		return ListResult{}, err
	}

	// Calculate pagination
	skip := (page - 1) * pageSize
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	// Find entries
	opts := options.Find().
		SetSort(bson.D{{Key: "started_at", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize))

	cur, err := s.c.Find(ctx, query, opts)
	if err != nil {
		return ListResult{}, err
	}
	defer cur.Close(ctx)

	var entries []Entry
	if err := cur.All(ctx, &entries); err != nil {
		return ListResult{}, err
	}

	return ListResult{
		Entries:    entries,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// buildQuery constructs a MongoDB query from ListFilter.
func (s *Store) buildQuery(filter ListFilter) bson.M {
	query := bson.M{}

	// Time range
	if filter.StartTime != nil || filter.EndTime != nil {
		timeQuery := bson.M{}
		if filter.StartTime != nil {
			timeQuery["$gte"] = *filter.StartTime
		}
		if filter.EndTime != nil {
			timeQuery["$lte"] = *filter.EndTime
		}
		query["started_at"] = timeQuery
	}

	// Actor filters
	if filter.ActorType != "" {
		query["actor_type"] = filter.ActorType
	}
	if filter.ActorID != "" {
		query["actor_id"] = filter.ActorID
	}

	// Request filters
	if filter.Method != "" {
		query["method"] = filter.Method
	}
	if filter.Path != "" {
		query["path"] = filter.Path
	} else if filter.PathPrefix != "" {
		query["path"] = bson.M{"$regex": "^" + filter.PathPrefix}
	}

	// Response filters
	if filter.StatusCodeMin != nil || filter.StatusCodeMax != nil {
		statusQuery := bson.M{}
		if filter.StatusCodeMin != nil {
			statusQuery["$gte"] = *filter.StatusCodeMin
		}
		if filter.StatusCodeMax != nil {
			statusQuery["$lte"] = *filter.StatusCodeMax
		}
		query["status_code"] = statusQuery
	}
	if filter.ErrorClass != "" {
		query["error_class"] = filter.ErrorClass
	}

	// Search
	if filter.Search != "" {
		query["$or"] = []bson.M{
			{"request_id": bson.M{"$regex": filter.Search, "$options": "i"}},
			{"path": bson.M{"$regex": filter.Search, "$options": "i"}},
			{"actor_name": bson.M{"$regex": filter.Search, "$options": "i"}},
		}
	}

	return query
}

// DeleteByDateRange deletes entries within a date range.
func (s *Store) DeleteByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	result, err := s.c.DeleteMany(ctx, bson.M{
		"started_at": bson.M{
			"$gte": start,
			"$lte": end,
		},
	})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// DeleteByRequestIDs deletes entries by their request IDs.
func (s *Store) DeleteByRequestIDs(ctx context.Context, requestIDs []string) (int64, error) {
	if len(requestIDs) == 0 {
		return 0, nil
	}
	result, err := s.c.DeleteMany(ctx, bson.M{
		"request_id": bson.M{"$in": requestIDs},
	})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// DeleteOlderThan deletes entries older than the specified duration.
func (s *Store) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.c.DeleteMany(ctx, bson.M{
		"started_at": bson.M{"$lt": cutoff},
	})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// CountByStatus returns counts grouped by status code ranges.
func (s *Store) CountByStatus(ctx context.Context, start, end time.Time) (map[string]int64, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"started_at": bson.M{"$gte": start, "$lte": end},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"$switch": bson.M{
						"branches": []bson.M{
							{"case": bson.M{"$lt": []any{"$status_code", 200}}, "then": "1xx"},
							{"case": bson.M{"$lt": []any{"$status_code", 300}}, "then": "2xx"},
							{"case": bson.M{"$lt": []any{"$status_code", 400}}, "then": "3xx"},
							{"case": bson.M{"$lt": []any{"$status_code", 500}}, "then": "4xx"},
						},
						"default": "5xx",
					},
				},
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cur, err := s.c.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	result := make(map[string]int64)
	for cur.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		result[doc.ID] = doc.Count
	}

	return result, nil
}

// AverageResponseTime returns the average response time in milliseconds.
func (s *Store) AverageResponseTime(ctx context.Context, start, end time.Time) (float64, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"started_at": bson.M{"$gte": start, "$lte": end},
			},
		},
		{
			"$group": bson.M{
				"_id":        nil,
				"avg_time":   bson.M{"$avg": "$timing.total_ms"},
				"total_reqs": bson.M{"$sum": 1},
			},
		},
	}

	cur, err := s.c.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)

	if cur.Next(ctx) {
		var doc struct {
			AvgTime float64 `bson:"avg_time"`
		}
		if err := cur.Decode(&doc); err != nil {
			return 0, err
		}
		return doc.AvgTime, nil
	}

	return 0, nil
}

// RecentErrors returns the most recent error entries.
func (s *Store) RecentErrors(ctx context.Context, limit int) ([]Entry, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "started_at", Value: -1}}).
		SetLimit(int64(limit))

	cur, err := s.c.Find(ctx, bson.M{
		"status_code": bson.M{"$gte": 400},
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var entries []Entry
	if err := cur.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

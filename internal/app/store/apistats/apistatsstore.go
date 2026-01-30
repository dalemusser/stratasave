// Package apistats provides storage for API request statistics with configurable bucket duration.
package apistats

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CollectionName is the MongoDB collection for API statistics.
const CollectionName = "api_stats"

// StatType identifies the type of API operation being tracked.
type StatType string

const (
	StatTypeSaveState    StatType = "state_save"
	StatTypeLoadState    StatType = "state_load"
	StatTypeSaveSettings StatType = "settings_save"
	StatTypeLoadSettings StatType = "settings_load"
)

// Bucket represents a time bucket of aggregated statistics.
type Bucket struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	Bucket         time.Time          `bson:"bucket"`          // Bucket start time
	BucketDuration string             `bson:"bucket_duration"` // Duration string (e.g., "1h", "15m")
	StatType       StatType           `bson:"stat_type"`       // Type of API operation
	Requests       int64              `bson:"requests"`        // Total request count
	Errors         int64              `bson:"errors"`          // Error count (4xx, 5xx)
	TotalMs        int64              `bson:"total_ms"`        // Sum of response times in ms
	MinMs          int64              `bson:"min_ms"`          // Minimum response time
	MaxMs          int64              `bson:"max_ms"`          // Maximum response time
	UpdatedAt      time.Time          `bson:"updated_at"`      // Last update time
}

// AvgMs returns the average response time in milliseconds.
func (b *Bucket) AvgMs() float64 {
	if b.Requests == 0 {
		return 0
	}
	return float64(b.TotalMs) / float64(b.Requests)
}

var (
	// ErrNotFound is returned when no stats are found.
	ErrNotFound = errors.New("stats not found")
)

// Store provides API statistics persistence.
type Store struct {
	c *mongo.Collection
}

// New creates a new API stats store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection(CollectionName)}
}

// EnsureIndexes creates indexes for efficient queries.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "bucket", Value: 1},
				{Key: "stat_type", Value: 1},
				{Key: "bucket_duration", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("idx_bucket_type_duration"),
		},
		{
			Keys: bson.D{
				{Key: "stat_type", Value: 1},
				{Key: "bucket", Value: 1},
			},
			Options: options.Index().SetName("idx_type_bucket"),
		},
	}
	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// TruncateToBucket truncates a time to the start of its bucket.
func TruncateToBucket(t time.Time, duration time.Duration) time.Time {
	return t.UTC().Truncate(duration)
}

// ParseBucketDuration parses a duration string like "1h", "15m", "24h".
func ParseBucketDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

// Record records a single API request's statistics.
// This atomically updates the appropriate bucket, creating it if needed.
func (s *Store) Record(ctx context.Context, statType StatType, bucketDuration time.Duration, durationMs int64, isError bool) error {
	now := time.Now().UTC()
	bucket := TruncateToBucket(now, bucketDuration)
	durationStr := bucketDuration.String()

	// Build the update
	// Note: $min and $max handle both insert (if field doesn't exist) and update cases,
	// so we don't include min_ms/max_ms in $setOnInsert (which would conflict).
	update := bson.M{
		"$inc": bson.M{
			"requests": 1,
			"total_ms": durationMs,
		},
		"$set": bson.M{
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"_id":             primitive.NewObjectID(),
			"bucket":          bucket,
			"bucket_duration": durationStr,
			"stat_type":       statType,
		},
		"$min": bson.M{
			"min_ms": durationMs,
		},
		"$max": bson.M{
			"max_ms": durationMs,
		},
	}

	if isError {
		update["$inc"].(bson.M)["errors"] = 1
	}

	opts := options.Update().SetUpsert(true)
	_, err := s.c.UpdateOne(ctx, bson.M{
		"bucket":          bucket,
		"stat_type":       statType,
		"bucket_duration": durationStr,
	}, update, opts)
	return err
}

// GetRange retrieves stats for a time range and stat type.
// If bucketDuration is empty, returns all resolutions.
func (s *Store) GetRange(ctx context.Context, statType StatType, startTime, endTime time.Time, bucketDuration string) ([]Bucket, error) {
	filter := bson.M{
		"stat_type": statType,
		"bucket": bson.M{
			"$gte": startTime.UTC(),
			"$lte": endTime.UTC(),
		},
	}
	if bucketDuration != "" {
		filter["bucket_duration"] = bucketDuration
	}

	opts := options.Find().SetSort(bson.D{{Key: "bucket", Value: 1}})
	cur, err := s.c.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var buckets []Bucket
	if err := cur.All(ctx, &buckets); err != nil {
		return nil, err
	}
	return buckets, nil
}

// GetRangeAllTypes retrieves stats for all stat types in a time range.
func (s *Store) GetRangeAllTypes(ctx context.Context, startTime, endTime time.Time, bucketDuration string) ([]Bucket, error) {
	filter := bson.M{
		"bucket": bson.M{
			"$gte": startTime.UTC(),
			"$lte": endTime.UTC(),
		},
	}
	if bucketDuration != "" {
		filter["bucket_duration"] = bucketDuration
	}

	opts := options.Find().SetSort(bson.D{
		{Key: "bucket", Value: 1},
		{Key: "stat_type", Value: 1},
	})
	cur, err := s.c.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var buckets []Bucket
	if err := cur.All(ctx, &buckets); err != nil {
		return nil, err
	}
	return buckets, nil
}

// GetDistinctDurations returns all distinct bucket durations in the collection.
func (s *Store) GetDistinctDurations(ctx context.Context) ([]string, error) {
	results, err := s.c.Distinct(ctx, "bucket_duration", bson.M{})
	if err != nil {
		return nil, err
	}

	durations := make([]string, 0, len(results))
	for _, r := range results {
		if d, ok := r.(string); ok {
			durations = append(durations, d)
		}
	}
	return durations, nil
}

// AggregateRange aggregates stats over a time range, combining all buckets.
func (s *Store) AggregateRange(ctx context.Context, statType StatType, startTime, endTime time.Time) (*AggregatedStats, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"stat_type": statType,
			"bucket": bson.M{
				"$gte": startTime.UTC(),
				"$lte": endTime.UTC(),
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":      nil,
			"requests": bson.M{"$sum": "$requests"},
			"errors":   bson.M{"$sum": "$errors"},
			"total_ms": bson.M{"$sum": "$total_ms"},
			"min_ms":   bson.M{"$min": "$min_ms"},
			"max_ms":   bson.M{"$max": "$max_ms"},
		}}},
	}

	cur, err := s.c.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	if !cur.Next(ctx) {
		return &AggregatedStats{}, nil
	}

	var result struct {
		Requests int64 `bson:"requests"`
		Errors   int64 `bson:"errors"`
		TotalMs  int64 `bson:"total_ms"`
		MinMs    int64 `bson:"min_ms"`
		MaxMs    int64 `bson:"max_ms"`
	}
	if err := cur.Decode(&result); err != nil {
		return nil, err
	}

	return &AggregatedStats{
		Requests: result.Requests,
		Errors:   result.Errors,
		TotalMs:  result.TotalMs,
		MinMs:    result.MinMs,
		MaxMs:    result.MaxMs,
	}, nil
}

// AggregatedStats represents aggregated statistics over a time range.
type AggregatedStats struct {
	Requests int64
	Errors   int64
	TotalMs  int64
	MinMs    int64
	MaxMs    int64
}

// AvgMs returns the average response time in milliseconds.
func (a *AggregatedStats) AvgMs() float64 {
	if a.Requests == 0 {
		return 0
	}
	return float64(a.TotalMs) / float64(a.Requests)
}

// ErrorRate returns the error rate as a percentage.
func (a *AggregatedStats) ErrorRate() float64 {
	if a.Requests == 0 {
		return 0
	}
	return float64(a.Errors) / float64(a.Requests) * 100
}

// RollUp aggregates fine-grained buckets into coarser buckets.
// For example, roll up 1-minute buckets into 1-hour buckets.
func (s *Store) RollUp(ctx context.Context, statType StatType, startTime, endTime time.Time, sourceDuration, targetDuration time.Duration) error {
	targetDurationStr := targetDuration.String()

	// Get all buckets in the source duration
	buckets, err := s.GetRange(ctx, statType, startTime, endTime, sourceDuration.String())
	if err != nil {
		return err
	}

	if len(buckets) == 0 {
		return nil
	}

	// Group buckets by target bucket time
	grouped := make(map[time.Time][]Bucket)
	for _, b := range buckets {
		targetBucket := TruncateToBucket(b.Bucket, targetDuration)
		grouped[targetBucket] = append(grouped[targetBucket], b)
	}

	// Create aggregated buckets
	for targetBucket, sourceBuckets := range grouped {
		var totalRequests, totalErrors, totalMs int64
		minMs := int64(^uint64(0) >> 1) // Max int64
		maxMs := int64(0)

		for _, b := range sourceBuckets {
			totalRequests += b.Requests
			totalErrors += b.Errors
			totalMs += b.TotalMs
			if b.MinMs < minMs {
				minMs = b.MinMs
			}
			if b.MaxMs > maxMs {
				maxMs = b.MaxMs
			}
		}

		// Upsert the aggregated bucket
		now := time.Now().UTC()
		opts := options.Update().SetUpsert(true)
		_, err := s.c.UpdateOne(ctx, bson.M{
			"bucket":          targetBucket,
			"stat_type":       statType,
			"bucket_duration": targetDurationStr,
		}, bson.M{
			"$set": bson.M{
				"requests":   totalRequests,
				"errors":     totalErrors,
				"total_ms":   totalMs,
				"min_ms":     minMs,
				"max_ms":     maxMs,
				"updated_at": now,
			},
			"$setOnInsert": bson.M{
				"_id": primitive.NewObjectID(),
			},
		}, opts)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteOlderThan deletes stats older than the cutoff time.
// If bucketDuration is specified, only deletes that resolution.
func (s *Store) DeleteOlderThan(ctx context.Context, cutoff time.Time, bucketDuration string) (int64, error) {
	filter := bson.M{
		"bucket": bson.M{"$lt": cutoff.UTC()},
	}
	if bucketDuration != "" {
		filter["bucket_duration"] = bucketDuration
	}

	result, err := s.c.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// Summary represents a summary of stats for a stat type.
type Summary struct {
	StatType       StatType
	TotalRequests  int64
	TotalErrors    int64
	AvgMs          float64
	MinMs          int64
	MaxMs          int64
	FirstBucket    time.Time
	LastBucket     time.Time
	BucketDuration string
}

// GetSummary returns a summary of stats for each stat type in the given range.
func (s *Store) GetSummary(ctx context.Context, startTime, endTime time.Time) ([]Summary, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"bucket": bson.M{
				"$gte": startTime.UTC(),
				"$lte": endTime.UTC(),
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":          "$stat_type",
			"requests":     bson.M{"$sum": "$requests"},
			"errors":       bson.M{"$sum": "$errors"},
			"total_ms":     bson.M{"$sum": "$total_ms"},
			"min_ms":       bson.M{"$min": "$min_ms"},
			"max_ms":       bson.M{"$max": "$max_ms"},
			"first_bucket": bson.M{"$min": "$bucket"},
			"last_bucket":  bson.M{"$max": "$bucket"},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	cur, err := s.c.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var summaries []Summary
	for cur.Next(ctx) {
		var doc struct {
			ID          string    `bson:"_id"`
			Requests    int64     `bson:"requests"`
			Errors      int64     `bson:"errors"`
			TotalMs     int64     `bson:"total_ms"`
			MinMs       int64     `bson:"min_ms"`
			MaxMs       int64     `bson:"max_ms"`
			FirstBucket time.Time `bson:"first_bucket"`
			LastBucket  time.Time `bson:"last_bucket"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}

		avgMs := float64(0)
		if doc.Requests > 0 {
			avgMs = float64(doc.TotalMs) / float64(doc.Requests)
		}

		summaries = append(summaries, Summary{
			StatType:      StatType(doc.ID),
			TotalRequests: doc.Requests,
			TotalErrors:   doc.Errors,
			AvgMs:         avgMs,
			MinMs:         doc.MinMs,
			MaxMs:         doc.MaxMs,
			FirstBucket:   doc.FirstBucket,
			LastBucket:    doc.LastBucket,
		})
	}

	return summaries, nil
}

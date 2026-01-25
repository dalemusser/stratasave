// internal/app/store/stats/statsstore.go
package statsstore

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DailyStats holds statistics for a single day.
type DailyStats struct {
	ID        primitive.ObjectID `bson:"_id"`
	Date      time.Time          `bson:"date"`      // Truncated to day (UTC midnight)
	StatType  string             `bson:"stat_type"` // "api_requests", "jobs", app-specific
	Counters  map[string]int64   `bson:"counters"`  // Flexible counters
	Gauges    map[string]float64 `bson:"gauges"`    // Avg response time, etc.
	UpdatedAt time.Time          `bson:"updated_at"`
}

var (
	// ErrNotFound is returned when stats are not found.
	ErrNotFound = errors.New("stats not found")
)

// Store provides statistics persistence.
type Store struct {
	c *mongo.Collection
}

// New creates a new stats store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("daily_stats")}
}

// truncateToDay returns the date truncated to midnight UTC.
func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// IncrementCounter atomically increments a counter for the given date and stat type.
func (s *Store) IncrementCounter(ctx context.Context, date time.Time, statType, counter string, delta int64) error {
	day := truncateToDay(date)
	now := time.Now()

	opts := options.Update().SetUpsert(true)
	_, err := s.c.UpdateOne(ctx, bson.M{
		"date":      day,
		"stat_type": statType,
	}, bson.M{
		"$inc": bson.M{
			"counters." + counter: delta,
		},
		"$set": bson.M{
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
		},
	}, opts)
	return err
}

// SetGauge sets a gauge value for the given date and stat type.
func (s *Store) SetGauge(ctx context.Context, date time.Time, statType, gauge string, value float64) error {
	day := truncateToDay(date)
	now := time.Now()

	opts := options.Update().SetUpsert(true)
	_, err := s.c.UpdateOne(ctx, bson.M{
		"date":      day,
		"stat_type": statType,
	}, bson.M{
		"$set": bson.M{
			"gauges." + gauge: value,
			"updated_at":      now,
		},
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
		},
	}, opts)
	return err
}

// SetCounters sets multiple counters at once.
func (s *Store) SetCounters(ctx context.Context, date time.Time, statType string, counters map[string]int64) error {
	day := truncateToDay(date)
	now := time.Now()

	set := bson.M{
		"updated_at": now,
	}
	for k, v := range counters {
		set["counters."+k] = v
	}

	opts := options.Update().SetUpsert(true)
	_, err := s.c.UpdateOne(ctx, bson.M{
		"date":      day,
		"stat_type": statType,
	}, bson.M{
		"$set": set,
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
		},
	}, opts)
	return err
}

// SetGauges sets multiple gauges at once.
func (s *Store) SetGauges(ctx context.Context, date time.Time, statType string, gauges map[string]float64) error {
	day := truncateToDay(date)
	now := time.Now()

	set := bson.M{
		"updated_at": now,
	}
	for k, v := range gauges {
		set["gauges."+k] = v
	}

	opts := options.Update().SetUpsert(true)
	_, err := s.c.UpdateOne(ctx, bson.M{
		"date":      day,
		"stat_type": statType,
	}, bson.M{
		"$set": set,
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
		},
	}, opts)
	return err
}

// GetForDate retrieves stats for a specific date and type.
func (s *Store) GetForDate(ctx context.Context, date time.Time, statType string) (*DailyStats, error) {
	day := truncateToDay(date)
	var stats DailyStats
	err := s.c.FindOne(ctx, bson.M{
		"date":      day,
		"stat_type": statType,
	}).Decode(&stats)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &stats, nil
}

// GetRange retrieves stats for a date range and type.
func (s *Store) GetRange(ctx context.Context, startDate, endDate time.Time, statType string) ([]DailyStats, error) {
	start := truncateToDay(startDate)
	end := truncateToDay(endDate).Add(24 * time.Hour) // Include the end date

	opts := options.Find().SetSort(bson.D{{Key: "date", Value: 1}})
	cur, err := s.c.Find(ctx, bson.M{
		"date":      bson.M{"$gte": start, "$lt": end},
		"stat_type": statType,
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var stats []DailyStats
	if err := cur.All(ctx, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetRangeAllTypes retrieves stats for a date range across all types.
func (s *Store) GetRangeAllTypes(ctx context.Context, startDate, endDate time.Time) ([]DailyStats, error) {
	start := truncateToDay(startDate)
	end := truncateToDay(endDate).Add(24 * time.Hour)

	opts := options.Find().SetSort(bson.D{
		{Key: "date", Value: 1},
		{Key: "stat_type", Value: 1},
	})
	cur, err := s.c.Find(ctx, bson.M{
		"date": bson.M{"$gte": start, "$lt": end},
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var stats []DailyStats
	if err := cur.All(ctx, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// SumCounters sums counters across a date range.
func (s *Store) SumCounters(ctx context.Context, startDate, endDate time.Time, statType string) (map[string]int64, error) {
	start := truncateToDay(startDate)
	end := truncateToDay(endDate).Add(24 * time.Hour)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"date":      bson.M{"$gte": start, "$lt": end},
				"stat_type": statType,
			},
		},
		{
			"$project": bson.M{
				"counters": bson.M{"$objectToArray": "$counters"},
			},
		},
		{
			"$unwind": "$counters",
		},
		{
			"$group": bson.M{
				"_id":   "$counters.k",
				"total": bson.M{"$sum": "$counters.v"},
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
			Key   string `bson:"_id"`
			Total int64  `bson:"total"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		result[doc.Key] = doc.Total
	}

	return result, nil
}

// AvgGauges averages gauges across a date range.
func (s *Store) AvgGauges(ctx context.Context, startDate, endDate time.Time, statType string) (map[string]float64, error) {
	start := truncateToDay(startDate)
	end := truncateToDay(endDate).Add(24 * time.Hour)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"date":      bson.M{"$gte": start, "$lt": end},
				"stat_type": statType,
			},
		},
		{
			"$project": bson.M{
				"gauges": bson.M{"$objectToArray": "$gauges"},
			},
		},
		{
			"$unwind": "$gauges",
		},
		{
			"$group": bson.M{
				"_id": "$gauges.k",
				"avg": bson.M{"$avg": "$gauges.v"},
			},
		},
	}

	cur, err := s.c.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	result := make(map[string]float64)
	for cur.Next(ctx) {
		var doc struct {
			Key string  `bson:"_id"`
			Avg float64 `bson:"avg"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		result[doc.Key] = doc.Avg
	}

	return result, nil
}

// DeleteOlderThan deletes stats older than the cutoff date.
func (s *Store) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	day := truncateToDay(cutoff)
	result, err := s.c.DeleteMany(ctx, bson.M{
		"date": bson.M{"$lt": day},
	})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// GetStatTypes returns all unique stat types.
func (s *Store) GetStatTypes(ctx context.Context) ([]string, error) {
	types, err := s.c.Distinct(ctx, "stat_type", bson.M{})
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(types))
	for _, t := range types {
		if str, ok := t.(string); ok {
			result = append(result, str)
		}
	}
	return result, nil
}

// CounterTimeSeries returns counter values over time for charting.
type CounterTimeSeries struct {
	Date  time.Time
	Value int64
}

// GetCounterTimeSeries returns a time series of a specific counter.
func (s *Store) GetCounterTimeSeries(ctx context.Context, startDate, endDate time.Time, statType, counter string) ([]CounterTimeSeries, error) {
	stats, err := s.GetRange(ctx, startDate, endDate, statType)
	if err != nil {
		return nil, err
	}

	result := make([]CounterTimeSeries, 0, len(stats))
	for _, stat := range stats {
		value := int64(0)
		if stat.Counters != nil {
			if v, ok := stat.Counters[counter]; ok {
				value = v
			}
		}
		result = append(result, CounterTimeSeries{
			Date:  stat.Date,
			Value: value,
		})
	}

	return result, nil
}

// GaugeTimeSeries returns gauge values over time for charting.
type GaugeTimeSeries struct {
	Date  time.Time
	Value float64
}

// GetGaugeTimeSeries returns a time series of a specific gauge.
func (s *Store) GetGaugeTimeSeries(ctx context.Context, startDate, endDate time.Time, statType, gauge string) ([]GaugeTimeSeries, error) {
	stats, err := s.GetRange(ctx, startDate, endDate, statType)
	if err != nil {
		return nil, err
	}

	result := make([]GaugeTimeSeries, 0, len(stats))
	for _, stat := range stats {
		value := float64(0)
		if stat.Gauges != nil {
			if v, ok := stat.Gauges[gauge]; ok {
				value = v
			}
		}
		result = append(result, GaugeTimeSeries{
			Date:  stat.Date,
			Value: value,
		})
	}

	return result, nil
}

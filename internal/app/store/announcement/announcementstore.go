// internal/app/store/announcement/announcementstore.go
package announcement

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Type represents the announcement type.
type Type string

const (
	TypeInfo     Type = "info"
	TypeWarning  Type = "warning"
	TypeCritical Type = "critical"
)

// Announcement represents a system announcement.
type Announcement struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Title       string             `bson:"title"`
	Content     string             `bson:"content"`
	Type        Type               `bson:"type"`
	Dismissible bool               `bson:"dismissible"`
	Active      bool               `bson:"active"`
	StartsAt    *time.Time         `bson:"starts_at,omitempty"`
	EndsAt      *time.Time         `bson:"ends_at,omitempty"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
}

// Store provides access to the announcements collection.
type Store struct {
	c *mongo.Collection
}

// New creates a new announcement store.
func New(db *mongo.Database) *Store {
	return &Store{
		c: db.Collection("announcements"),
	}
}

// EnsureIndexes creates necessary indexes for the collection.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "active", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "starts_at", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "ends_at", Value: 1}},
			Options: options.Index(),
		},
	}

	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// CreateInput contains the input for creating an announcement.
type CreateInput struct {
	Title       string
	Content     string
	Type        Type
	Dismissible bool
	Active      bool
	StartsAt    *time.Time
	EndsAt      *time.Time
}

// Create creates a new announcement.
func (s *Store) Create(ctx context.Context, input CreateInput) (*Announcement, error) {
	now := time.Now()
	ann := Announcement{
		ID:          primitive.NewObjectID(),
		Title:       input.Title,
		Content:     input.Content,
		Type:        input.Type,
		Dismissible: input.Dismissible,
		Active:      input.Active,
		StartsAt:    input.StartsAt,
		EndsAt:      input.EndsAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if _, err := s.c.InsertOne(ctx, ann); err != nil {
		return nil, err
	}

	return &ann, nil
}

// GetByID retrieves an announcement by ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*Announcement, error) {
	var ann Announcement
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&ann); err != nil {
		return nil, err
	}
	return &ann, nil
}

// UpdateInput contains the input for updating an announcement.
type UpdateInput struct {
	Title       *string
	Content     *string
	Type        *Type
	Dismissible *bool
	Active      *bool
	StartsAt    *time.Time
	EndsAt      *time.Time
}

// Update updates an announcement.
func (s *Store) Update(ctx context.Context, id primitive.ObjectID, input UpdateInput) error {
	set := bson.M{"updated_at": time.Now()}

	if input.Title != nil {
		set["title"] = *input.Title
	}
	if input.Content != nil {
		set["content"] = *input.Content
	}
	if input.Type != nil {
		set["type"] = *input.Type
	}
	if input.Dismissible != nil {
		set["dismissible"] = *input.Dismissible
	}
	if input.Active != nil {
		set["active"] = *input.Active
	}
	if input.StartsAt != nil {
		set["starts_at"] = *input.StartsAt
	}
	if input.EndsAt != nil {
		set["ends_at"] = *input.EndsAt
	}

	_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

// Delete deletes an announcement.
func (s *Store) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := s.c.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// List returns all announcements, sorted by creation date descending.
func (s *Store) List(ctx context.Context) ([]Announcement, error) {
	cursor, err := s.c.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var announcements []Announcement
	if err := cursor.All(ctx, &announcements); err != nil {
		return nil, err
	}

	return announcements, nil
}

// GetActive returns all currently active announcements that should be displayed.
// This performs all time-based filtering in MongoDB for efficiency.
func (s *Store) GetActive(ctx context.Context) ([]Announcement, error) {
	now := time.Now()
	// Filter in MongoDB: active=true, starts_at is null or <= now, ends_at is null or > now
	filter := bson.M{
		"active": true,
		"$and": []bson.M{
			// starts_at condition: null or started
			{"$or": []bson.M{
				{"starts_at": nil},
				{"starts_at": bson.M{"$lte": now}},
			}},
			// ends_at condition: null or not yet ended
			{"$or": []bson.M{
				{"ends_at": nil},
				{"ends_at": bson.M{"$gt": now}},
			}},
		},
	}

	cursor, err := s.c.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "type", Value: -1}, {Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var announcements []Announcement
	if err := cursor.All(ctx, &announcements); err != nil {
		return nil, err
	}

	return announcements, nil
}

// SetActive sets the active status of an announcement.
func (s *Store) SetActive(ctx context.Context, id primitive.ObjectID, active bool) error {
	_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"active":     active,
			"updated_at": time.Now(),
		},
	})
	return err
}

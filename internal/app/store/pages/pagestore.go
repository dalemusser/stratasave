// internal/app/store/pages/pagestore.go
package pagestore

import (
	"context"
	"time"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store provides access to the pages collection.
type Store struct {
	c *mongo.Collection
}

// New creates a new page store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("pages")}
}

// GetBySlug returns a page by its slug.
func (s *Store) GetBySlug(ctx context.Context, slug string) (models.Page, error) {
	var page models.Page
	err := s.c.FindOne(ctx, bson.M{"slug": slug}).Decode(&page)
	if err != nil {
		return models.Page{}, err
	}
	return page, nil
}

// Upsert creates or updates a page by slug.
// If a page with the given slug exists, it updates it; otherwise creates a new one.
func (s *Store) Upsert(ctx context.Context, page models.Page) error {
	now := time.Now().UTC()
	page.UpdatedAt = &now

	filter := bson.M{"slug": page.Slug}
	update := bson.M{
		"$set": bson.M{
			"title":           page.Title,
			"content":         page.Content,
			"updated_at":      page.UpdatedAt,
			"updated_by_id":   page.UpdatedByID,
			"updated_by_name": page.UpdatedByName,
		},
		"$setOnInsert": bson.M{
			"_id":  primitive.NewObjectID(),
			"slug": page.Slug,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := s.c.UpdateOne(ctx, filter, update, opts)
	return err
}

// GetAll returns all pages.
func (s *Store) GetAll(ctx context.Context) ([]models.Page, error) {
	cur, err := s.c.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var pages []models.Page
	if err := cur.All(ctx, &pages); err != nil {
		return nil, err
	}
	return pages, nil
}

// Exists checks if a page with the given slug exists.
func (s *Store) Exists(ctx context.Context, slug string) (bool, error) {
	count, err := s.c.CountDocuments(ctx, bson.M{"slug": slug})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

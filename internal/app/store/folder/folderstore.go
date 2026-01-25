// Package folder provides storage for file folders.
package folder

import (
	"context"
	"time"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/text"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store provides access to the file_folders collection.
type Store struct {
	c *mongo.Collection
}

// New creates a new folder store.
func New(db *mongo.Database) *Store {
	return &Store{
		c: db.Collection("file_folders"),
	}
}

// CreateInput contains the input for creating a folder.
type CreateInput struct {
	Name        string
	ParentID    *primitive.ObjectID
	Description string
	CreatedByID primitive.ObjectID
}

// Create creates a new folder.
func (s *Store) Create(ctx context.Context, input CreateInput) (*models.Folder, error) {
	now := time.Now()
	folder := models.Folder{
		ID:          primitive.NewObjectID(),
		Name:        input.Name,
		NameCI:      text.Fold(input.Name),
		ParentID:    input.ParentID,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedByID: input.CreatedByID,
	}

	if _, err := s.c.InsertOne(ctx, folder); err != nil {
		return nil, err
	}

	return &folder, nil
}

// GetByID retrieves a folder by ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Folder, error) {
	var folder models.Folder
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

// UpdateInput contains the input for updating a folder.
type UpdateInput struct {
	Name        *string
	Description *string
}

// Update updates a folder.
func (s *Store) Update(ctx context.Context, id primitive.ObjectID, input UpdateInput) error {
	set := bson.M{"updated_at": time.Now()}

	if input.Name != nil {
		set["name"] = *input.Name
		set["name_ci"] = text.Fold(*input.Name)
	}
	if input.Description != nil {
		set["description"] = *input.Description
	}

	_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

// Delete deletes a folder.
func (s *Store) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := s.c.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// ListOptions contains options for listing folders.
type ListOptions struct {
	SortBy    string // "name", "created_at", "updated_at"
	SortOrder int    // 1 = asc, -1 = desc
}

// ListByParent returns all folders within a parent folder.
// Pass nil for parentID to list root folders.
func (s *Store) ListByParent(ctx context.Context, parentID *primitive.ObjectID, opts ListOptions) ([]models.Folder, error) {
	filter := bson.M{"parent_id": parentID}

	// Determine sort field
	sortField := "name_ci"
	switch opts.SortBy {
	case "created_at", "date":
		sortField = "created_at"
	case "updated_at":
		sortField = "updated_at"
	}

	sortOrder := 1
	if opts.SortOrder != 0 {
		sortOrder = opts.SortOrder
	}

	findOpts := options.Find().SetSort(bson.D{{Key: sortField, Value: sortOrder}})

	cursor, err := s.c.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var folders []models.Folder
	if err := cursor.All(ctx, &folders); err != nil {
		return nil, err
	}

	return folders, nil
}

// CountByParent returns the number of folders within a parent folder.
func (s *Store) CountByParent(ctx context.Context, parentID *primitive.ObjectID) (int64, error) {
	return s.c.CountDocuments(ctx, bson.M{"parent_id": parentID})
}

// GetAncestors returns all ancestors of a folder, ordered from root to immediate parent.
func (s *Store) GetAncestors(ctx context.Context, id primitive.ObjectID) ([]models.Folder, error) {
	// First get the folder to find its parent
	folder, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var ancestors []models.Folder

	// Walk up the parent chain
	currentParentID := folder.ParentID
	for currentParentID != nil {
		parent, err := s.GetByID(ctx, *currentParentID)
		if err != nil {
			return nil, err
		}
		// Prepend to get root-first order
		ancestors = append([]models.Folder{*parent}, ancestors...)
		currentParentID = parent.ParentID
	}

	return ancestors, nil
}

// GetPath returns the full path of a folder (ancestors + the folder itself).
func (s *Store) GetPath(ctx context.Context, id primitive.ObjectID) ([]models.Folder, error) {
	folder, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	ancestors, err := s.GetAncestors(ctx, id)
	if err != nil {
		return nil, err
	}

	// Append the folder itself
	return append(ancestors, *folder), nil
}

// NameExistsInParent checks if a folder with the given name exists in the parent.
// Pass excludeID to exclude a specific folder (useful for updates).
func (s *Store) NameExistsInParent(ctx context.Context, name string, parentID *primitive.ObjectID, excludeID *primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"parent_id": parentID,
		"name_ci":   text.Fold(name),
	}

	if excludeID != nil {
		filter["_id"] = bson.M{"$ne": *excludeID}
	}

	count, err := s.c.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// HasSubfolders checks if a folder has any subfolders.
func (s *Store) HasSubfolders(ctx context.Context, id primitive.ObjectID) (bool, error) {
	count, err := s.c.CountDocuments(ctx, bson.M{"parent_id": id})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

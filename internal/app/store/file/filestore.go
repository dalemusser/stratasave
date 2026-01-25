// Package file provides storage for files metadata.
package file

import (
	"context"
	"strings"
	"time"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/text"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store provides access to the files collection.
type Store struct {
	c *mongo.Collection
}

// New creates a new file store.
func New(db *mongo.Database) *Store {
	return &Store{
		c: db.Collection("files"),
	}
}

// CreateInput contains the input for creating a file.
type CreateInput struct {
	FolderID    *primitive.ObjectID
	Name        string
	StoragePath string
	Size        int64
	ContentType string
	Description string
	CreatedByID primitive.ObjectID
}

// Create creates a new file record.
func (s *Store) Create(ctx context.Context, input CreateInput) (*models.File, error) {
	now := time.Now()
	file := models.File{
		ID:          primitive.NewObjectID(),
		FolderID:    input.FolderID,
		Name:        input.Name,
		NameCI:      text.Fold(input.Name),
		StoragePath: input.StoragePath,
		Size:        input.Size,
		ContentType: input.ContentType,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedByID: input.CreatedByID,
	}

	if _, err := s.c.InsertOne(ctx, file); err != nil {
		return nil, err
	}

	return &file, nil
}

// GetByID retrieves a file by ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*models.File, error) {
	var file models.File
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&file); err != nil {
		return nil, err
	}
	return &file, nil
}

// UpdateInput contains the input for updating a file.
type UpdateInput struct {
	Name        *string
	Description *string
}

// Update updates a file.
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

// Delete deletes a file record.
func (s *Store) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := s.c.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// ListOptions contains options for listing files.
type ListOptions struct {
	SortBy      string // "name", "created_at", "size", "content_type"
	SortOrder   int    // 1 = asc, -1 = desc
	ContentType string // Filter by MIME type: prefix match (e.g., "image/") or contains match with ~ prefix (e.g., "~word,document")
	Search      string // Filter by filename
}

// ListByFolder returns all files within a folder.
// Pass nil for folderID to list root-level files.
func (s *Store) ListByFolder(ctx context.Context, folderID *primitive.ObjectID, opts ListOptions) ([]models.File, error) {
	filter := bson.M{"folder_id": folderID}

	// Apply content type filter
	if opts.ContentType != "" {
		if strings.HasPrefix(opts.ContentType, "~") {
			// Contains matching: ~word,document means contains "word" OR "document"
			terms := strings.Split(opts.ContentType[1:], ",")
			var orConditions []bson.M
			for _, term := range terms {
				term = strings.TrimSpace(term)
				if term != "" {
					orConditions = append(orConditions, bson.M{
						"content_type": bson.M{"$regex": term, "$options": "i"},
					})
				}
			}
			if len(orConditions) > 0 {
				filter["$or"] = orConditions
			}
		} else {
			// Prefix matching (existing behavior)
			filter["content_type"] = bson.M{"$regex": "^" + opts.ContentType}
		}
	}

	// Apply search filter
	if opts.Search != "" {
		searchFolded := text.Fold(opts.Search)
		filter["name_ci"] = bson.M{"$regex": searchFolded}
	}

	// Determine sort field
	sortField := "name_ci"
	switch opts.SortBy {
	case "created_at", "date":
		sortField = "created_at"
	case "size":
		sortField = "size"
	case "content_type", "type":
		sortField = "content_type"
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

	var files []models.File
	if err := cursor.All(ctx, &files); err != nil {
		return nil, err
	}

	return files, nil
}

// CountByFolder returns the number of files in a folder.
func (s *Store) CountByFolder(ctx context.Context, folderID *primitive.ObjectID) (int64, error) {
	return s.c.CountDocuments(ctx, bson.M{"folder_id": folderID})
}

// CountByFolderID returns the number of files in a specific folder (by ID, not pointer).
func (s *Store) CountByFolderID(ctx context.Context, folderID primitive.ObjectID) (int64, error) {
	return s.c.CountDocuments(ctx, bson.M{"folder_id": folderID})
}

// NameExistsInFolder checks if a file with the given name exists in the folder.
// Pass excludeID to exclude a specific file (useful for updates).
func (s *Store) NameExistsInFolder(ctx context.Context, name string, folderID *primitive.ObjectID, excludeID *primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"folder_id": folderID,
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

// DeleteByFolderID deletes all files in a folder.
func (s *Store) DeleteByFolderID(ctx context.Context, folderID primitive.ObjectID) (int64, error) {
	result, err := s.c.DeleteMany(ctx, bson.M{"folder_id": folderID})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// GetByFolderID returns all files in a specific folder.
func (s *Store) GetByFolderID(ctx context.Context, folderID primitive.ObjectID) ([]models.File, error) {
	cursor, err := s.c.Find(ctx, bson.M{"folder_id": folderID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var files []models.File
	if err := cursor.All(ctx, &files); err != nil {
		return nil, err
	}

	return files, nil
}

// FileTypeCategory returns a category string for a content type.
func FileTypeCategory(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	case contentType == "application/pdf":
		return "pdf"
	case strings.Contains(contentType, "spreadsheet") || strings.Contains(contentType, "excel"):
		return "spreadsheet"
	case strings.Contains(contentType, "document") || strings.Contains(contentType, "word"):
		return "document"
	case strings.Contains(contentType, "presentation") || strings.Contains(contentType, "powerpoint"):
		return "presentation"
	case strings.Contains(contentType, "zip") || strings.Contains(contentType, "compressed") || strings.Contains(contentType, "archive"):
		return "archive"
	default:
		return "file"
	}
}

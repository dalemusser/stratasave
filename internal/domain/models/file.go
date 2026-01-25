package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// File represents a file in the file system.
type File struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty"`
	FolderID    *primitive.ObjectID `bson:"folder_id,omitempty"` // nil = root level
	Name        string              `bson:"name"`                // Original filename
	NameCI      string              `bson:"name_ci"`             // Case-insensitive for sorting/search
	StoragePath string              `bson:"storage_path"`        // Path in storage backend
	Size        int64               `bson:"size"`                // File size in bytes
	ContentType string              `bson:"content_type"`        // MIME type
	Description string              `bson:"description,omitempty"`
	CreatedAt   time.Time           `bson:"created_at"`
	UpdatedAt   time.Time           `bson:"updated_at"`
	CreatedByID primitive.ObjectID  `bson:"created_by_id"`
}

// IsInRoot returns true if the file is at the root level (not in any folder).
func (f *File) IsInRoot() bool {
	return f.FolderID == nil
}

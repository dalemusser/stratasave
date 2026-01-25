package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Folder represents a folder in the file system.
type Folder struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty"`
	Name        string              `bson:"name"`
	NameCI      string              `bson:"name_ci"`             // Case-insensitive for sorting/search
	ParentID    *primitive.ObjectID `bson:"parent_id,omitempty"` // nil = root folder
	Description string              `bson:"description,omitempty"`
	CreatedAt   time.Time           `bson:"created_at"`
	UpdatedAt   time.Time           `bson:"updated_at"`
	CreatedByID primitive.ObjectID  `bson:"created_by_id"`
}

// IsRoot returns true if the folder is at the root level.
func (f *Folder) IsRoot() bool {
	return f.ParentID == nil
}

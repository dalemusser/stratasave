// internal/domain/models/page.go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Page represents editable content pages like About, Contact, Terms of Service, and Privacy Policy.
type Page struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Slug    string             `bson:"slug" json:"slug"`       // URL slug: "about", "contact", "terms", "privacy"
	Title   string             `bson:"title" json:"title"`     // Display title
	Content string             `bson:"content" json:"content"` // HTML content from TipTap editor

	// Audit fields
	UpdatedAt     *time.Time          `bson:"updated_at,omitempty" json:"updated_at,omitempty"`
	UpdatedByID   *primitive.ObjectID `bson:"updated_by_id,omitempty" json:"updated_by_id,omitempty"`
	UpdatedByName string              `bson:"updated_by_name,omitempty" json:"updated_by_name,omitempty"`
}

// Page slugs
const (
	PageSlugAbout   = "about"
	PageSlugContact = "contact"
	PageSlugTerms   = "terms"
	PageSlugPrivacy = "privacy"
)

// AllPageSlugs returns all valid page slugs.
func AllPageSlugs() []string {
	return []string{
		PageSlugAbout,
		PageSlugContact,
		PageSlugTerms,
		PageSlugPrivacy,
	}
}

// IsValidPageSlug checks if a slug is valid.
func IsValidPageSlug(slug string) bool {
	for _, s := range AllPageSlugs() {
		if s == slug {
			return true
		}
	}
	return false
}

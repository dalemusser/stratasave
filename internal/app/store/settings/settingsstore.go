// internal/app/store/settings/settingsstore.go
package settingsstore

import (
	"context"
	"time"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store provides access to the site_settings collection.
// Strata uses a singleton settings document (only one per site).
type Store struct {
	c *mongo.Collection
}

// New creates a new settings store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("site_settings")}
}

// Get returns the site settings.
// If no settings exist, returns default settings.
func (s *Store) Get(ctx context.Context) (*models.SiteSettings, error) {
	var settings models.SiteSettings
	// Use singleton filter - there's only one settings document
	filter := bson.M{"singleton": true}
	err := s.c.FindOne(ctx, filter).Decode(&settings)
	if err == mongo.ErrNoDocuments {
		// Return default settings
		return &models.SiteSettings{
			SiteName:       models.DefaultSiteName,
			LandingTitle:   models.DefaultLandingTitle,
			LandingContent: models.DefaultLandingContent,
			FooterHTML:     models.DefaultFooterHTML,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// Save updates the site settings.
// Uses upsert so it works whether settings exist or not.
func (s *Store) Save(ctx context.Context, settings models.SiteSettings) error {
	now := time.Now().UTC()
	settings.UpdatedAt = &now

	// Use singleton filter
	filter := bson.M{"singleton": true}
	update := bson.M{
		"$set": bson.M{
			"singleton":            true,
			"site_name":            settings.SiteName,
			"logo_path":            settings.LogoPath,
			"logo_name":            settings.LogoName,
			"landing_title":        settings.LandingTitle,
			"landing_content":      settings.LandingContent,
			"footer_html":          settings.FooterHTML,
			"enabled_auth_methods": settings.EnabledAuthMethods,
			"updated_at":           settings.UpdatedAt,
			"updated_by_id":        settings.UpdatedByID,
			"updated_by_name":      settings.UpdatedByName,
		},
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := s.c.UpdateOne(ctx, filter, update, opts)
	return err
}

// Exists checks if settings have been saved.
func (s *Store) Exists(ctx context.Context) (bool, error) {
	filter := bson.M{"singleton": true}
	count, err := s.c.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateInput holds the fields for updating settings.
type UpdateInput struct {
	SiteName       string
	LandingTitle   string
	LandingContent string
	FooterHTML     string
	LogoPath       string
	LogoName       string
	// Email notification settings
	NotifyUserOnCreate  bool
	NotifyUserOnDisable bool
	NotifyUserOnEnable  bool
	NotifyUserOnWelcome bool
}

// Upsert updates or inserts site settings from UpdateInput.
func (s *Store) Upsert(ctx context.Context, input UpdateInput) error {
	now := time.Now().UTC()

	filter := bson.M{"singleton": true}
	update := bson.M{
		"$set": bson.M{
			"singleton":              true,
			"site_name":              input.SiteName,
			"landing_title":          input.LandingTitle,
			"landing_content":        input.LandingContent,
			"footer_html":            input.FooterHTML,
			"logo_path":              input.LogoPath,
			"logo_name":              input.LogoName,
			"notify_user_on_create":  input.NotifyUserOnCreate,
			"notify_user_on_disable": input.NotifyUserOnDisable,
			"notify_user_on_enable":  input.NotifyUserOnEnable,
			"notify_user_on_welcome": input.NotifyUserOnWelcome,
			"updated_at":             now,
		},
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := s.c.UpdateOne(ctx, filter, update, opts)
	return err
}

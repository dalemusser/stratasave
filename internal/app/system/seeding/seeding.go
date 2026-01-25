// internal/app/system/seeding/seeding.go
package seeding

import (
	"context"

	pagestore "github.com/dalemusser/stratasave/internal/app/store/pages"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// SeedAll seeds default data if not already present.
func SeedAll(ctx context.Context, db *mongo.Database, logger *zap.Logger) error {
	if err := seedPages(ctx, db, logger); err != nil {
		return err
	}
	return nil
}

// seedPages creates default pages if they don't exist.
func seedPages(ctx context.Context, db *mongo.Database, logger *zap.Logger) error {
	store := pagestore.New(db)

	defaultPages := []models.Page{
		{
			Slug:  models.PageSlugAbout,
			Title: "About",
			Content: `<h2>About Us</h2>
<p>Welcome to our platform. This page can be customized by an administrator.</p>
<p>Use the edit button to update this content with information about your organization.</p>`,
		},
		{
			Slug:  models.PageSlugContact,
			Title: "Contact",
			Content: `<h2>Contact Us</h2>
<p>We'd love to hear from you. This page can be customized by an administrator.</p>
<p>Add your contact information, email addresses, phone numbers, or a contact form here.</p>`,
		},
		{
			Slug:  models.PageSlugTerms,
			Title: "Terms of Service",
			Content: `<h2>Terms of Service</h2>
<p>This page should contain your Terms of Service. An administrator should update this content.</p>
<p>Terms of Service typically include:</p>
<ul>
<li>Acceptance of terms</li>
<li>Description of service</li>
<li>User responsibilities</li>
<li>Intellectual property rights</li>
<li>Limitation of liability</li>
<li>Governing law</li>
</ul>`,
		},
		{
			Slug:  models.PageSlugPrivacy,
			Title: "Privacy Policy",
			Content: `<h2>Privacy Policy</h2>
<p>This page should contain your Privacy Policy. An administrator should update this content.</p>
<p>A Privacy Policy typically includes:</p>
<ul>
<li>What information is collected</li>
<li>How information is used</li>
<li>How information is protected</li>
<li>Cookie policy</li>
<li>Third-party sharing</li>
<li>User rights</li>
<li>Contact information for privacy concerns</li>
</ul>`,
		},
	}

	for _, page := range defaultPages {
		exists, err := store.Exists(ctx, page.Slug)
		if err != nil {
			logger.Error("failed to check if page exists",
				zap.String("slug", page.Slug),
				zap.Error(err))
			return err
		}
		if !exists {
			if err := store.Upsert(ctx, page); err != nil {
				logger.Error("failed to seed page",
					zap.String("slug", page.Slug),
					zap.Error(err))
				return err
			}
			logger.Info("seeded default page", zap.String("slug", page.Slug))
		}
	}

	return nil
}

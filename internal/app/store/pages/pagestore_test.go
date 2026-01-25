package pagestore

import (
	"testing"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestNew(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	if store == nil {
		t.Fatal("New() returned nil")
	}
}

func TestStore_Upsert_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	page := models.Page{
		Slug:          "about",
		Title:         "About Us",
		Content:       "<p>Welcome to our site</p>",
		UpdatedByID:   &userID,
		UpdatedByName: "Admin User",
	}

	err := store.Upsert(ctx, page)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	// Verify
	retrieved, err := store.GetBySlug(ctx, "about")
	if err != nil {
		t.Fatalf("GetBySlug() error = %v", err)
	}

	if retrieved.Slug != page.Slug {
		t.Errorf("Slug = %v, want %v", retrieved.Slug, page.Slug)
	}
	if retrieved.Title != page.Title {
		t.Errorf("Title = %v, want %v", retrieved.Title, page.Title)
	}
	if retrieved.Content != page.Content {
		t.Errorf("Content = %v, want %v", retrieved.Content, page.Content)
	}
	if retrieved.UpdatedByName != page.UpdatedByName {
		t.Errorf("UpdatedByName = %v, want %v", retrieved.UpdatedByName, page.UpdatedByName)
	}
	if retrieved.UpdatedAt == nil {
		t.Error("UpdatedAt should be set")
	}
}

func TestStore_Upsert_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()

	// Create initial page
	initial := models.Page{
		Slug:          "terms",
		Title:         "Terms of Service",
		Content:       "<p>Original terms</p>",
		UpdatedByID:   &userID,
		UpdatedByName: "User One",
	}
	store.Upsert(ctx, initial)

	// Update the same slug
	updated := models.Page{
		Slug:          "terms",
		Title:         "Updated Terms",
		Content:       "<p>Updated terms content</p>",
		UpdatedByID:   &userID,
		UpdatedByName: "User Two",
	}

	err := store.Upsert(ctx, updated)
	if err != nil {
		t.Fatalf("Upsert() update error = %v", err)
	}

	// Verify update
	retrieved, _ := store.GetBySlug(ctx, "terms")
	if retrieved.Title != updated.Title {
		t.Errorf("Title = %v, want %v", retrieved.Title, updated.Title)
	}
	if retrieved.Content != updated.Content {
		t.Errorf("Content = %v, want %v", retrieved.Content, updated.Content)
	}
	if retrieved.UpdatedByName != updated.UpdatedByName {
		t.Errorf("UpdatedByName = %v, want %v", retrieved.UpdatedByName, updated.UpdatedByName)
	}

	// Should still only be one page with this slug
	exists, _ := store.Exists(ctx, "terms")
	if !exists {
		t.Error("Page should exist")
	}

	all, _ := store.GetAll(ctx)
	count := 0
	for _, p := range all {
		if p.Slug == "terms" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Should have exactly 1 page with slug 'terms', got %d", count)
	}
}

func TestStore_GetBySlug(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a page
	store.Upsert(ctx, models.Page{
		Slug:    "contact",
		Title:   "Contact Us",
		Content: "<p>Get in touch</p>",
	})

	// Valid slug
	page, err := store.GetBySlug(ctx, "contact")
	if err != nil {
		t.Fatalf("GetBySlug() error = %v", err)
	}
	if page.Slug != "contact" {
		t.Errorf("Slug = %v, want 'contact'", page.Slug)
	}
	if page.Title != "Contact Us" {
		t.Errorf("Title = %v, want 'Contact Us'", page.Title)
	}

	// Invalid slug
	_, err = store.GetBySlug(ctx, "nonexistent")
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetBySlug() for nonexistent slug error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_GetAll(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create multiple pages
	slugs := []string{"about", "contact", "terms", "privacy"}
	for _, slug := range slugs {
		store.Upsert(ctx, models.Page{
			Slug:    slug,
			Title:   "Title for " + slug,
			Content: "<p>Content for " + slug + "</p>",
		})
	}

	pages, err := store.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(pages) != 4 {
		t.Errorf("GetAll() count = %d, want 4", len(pages))
	}

	// Verify all slugs are present
	foundSlugs := make(map[string]bool)
	for _, p := range pages {
		foundSlugs[p.Slug] = true
	}
	for _, slug := range slugs {
		if !foundSlugs[slug] {
			t.Errorf("GetAll() missing page with slug %q", slug)
		}
	}
}

func TestStore_GetAll_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	pages, err := store.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	// Note: MongoDB cursor.All returns nil for empty results
	if len(pages) != 0 {
		t.Errorf("GetAll() for empty collection should return 0, got %d", len(pages))
	}
}

func TestStore_Exists(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Initially should not exist
	exists, err := store.Exists(ctx, "about")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() should return false before page is created")
	}

	// Create the page
	store.Upsert(ctx, models.Page{
		Slug:    "about",
		Title:   "About",
		Content: "Content",
	})

	// Now should exist
	exists, err = store.Exists(ctx, "about")
	if err != nil {
		t.Fatalf("Exists() after create error = %v", err)
	}
	if !exists {
		t.Error("Exists() should return true after page is created")
	}

	// Different slug should not exist
	exists, _ = store.Exists(ctx, "different")
	if exists {
		t.Error("Exists() should return false for different slug")
	}
}

func TestPageModel_Constants(t *testing.T) {
	if models.PageSlugAbout != "about" {
		t.Errorf("PageSlugAbout = %q, want 'about'", models.PageSlugAbout)
	}
	if models.PageSlugContact != "contact" {
		t.Errorf("PageSlugContact = %q, want 'contact'", models.PageSlugContact)
	}
	if models.PageSlugTerms != "terms" {
		t.Errorf("PageSlugTerms = %q, want 'terms'", models.PageSlugTerms)
	}
	if models.PageSlugPrivacy != "privacy" {
		t.Errorf("PageSlugPrivacy = %q, want 'privacy'", models.PageSlugPrivacy)
	}
}

func TestPageModel_AllPageSlugs(t *testing.T) {
	slugs := models.AllPageSlugs()
	if len(slugs) != 4 {
		t.Errorf("AllPageSlugs() count = %d, want 4", len(slugs))
	}

	expected := map[string]bool{
		"about":   true,
		"contact": true,
		"terms":   true,
		"privacy": true,
	}

	for _, slug := range slugs {
		if !expected[slug] {
			t.Errorf("AllPageSlugs() contains unexpected slug %q", slug)
		}
	}
}

func TestPageModel_IsValidPageSlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"about", true},
		{"contact", true},
		{"terms", true},
		{"privacy", true},
		{"invalid", false},
		{"", false},
		{"About", false}, // case sensitive
		{"CONTACT", false},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := models.IsValidPageSlug(tt.slug)
			if got != tt.want {
				t.Errorf("IsValidPageSlug(%q) = %v, want %v", tt.slug, got, tt.want)
			}
		})
	}
}

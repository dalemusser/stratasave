// Package testutil provides utilities for testing, including database setup and fixtures.
package testutil

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/indexes"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// TestDBURI is the MongoDB connection string for tests.
	TestDBURI = "mongodb://localhost:27017"
	// TestDBName is the database name used for tests.
	TestDBName = "stratasave_test"
)

var (
	clientOnce sync.Once
	client     *mongo.Client
	clientErr  error
)

// getClient returns a shared MongoDB client for all tests.
// The client is created once and reused across tests.
func getClient() (*mongo.Client, error) {
	clientOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Configure connection pool for parallel test execution
		clientOpts := options.Client().
			ApplyURI(TestDBURI).
			SetMaxPoolSize(200).                  // Increase pool for parallel tests
			SetMinPoolSize(10).                   // Keep some connections warm
			SetMaxConnIdleTime(30 * time.Second). // Release idle connections faster
			SetConnectTimeout(10 * time.Second).  // Connection timeout
			SetServerSelectionTimeout(10 * time.Second)

		client, clientErr = mongo.Connect(ctx, clientOpts)
		if clientErr != nil {
			return
		}

		// Verify connection
		clientErr = client.Ping(ctx, nil)
	})
	return client, clientErr
}

// SetupTestDB returns a test database instance with indexes created.
// Each test gets a unique database based on the test name to avoid conflicts
// when running tests in parallel across packages.
// The database is dropped when the test completes via t.Cleanup.
func SetupTestDB(t *testing.T) *mongo.Database {
	t.Helper()

	client, err := getClient()
	if err != nil {
		t.Fatalf("failed to connect to test MongoDB: %v", err)
	}

	// Create unique database name from test name to avoid conflicts
	dbName := fmt.Sprintf("%s_%s", TestDBName, sanitizeTestName(t.Name()))
	db := client.Database(dbName)

	// Drop database to ensure clean state
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.Drop(ctx); err != nil {
		t.Fatalf("failed to drop test database: %v", err)
	}

	// Create indexes to match production behavior
	if err := indexes.EnsureAll(ctx, db); err != nil {
		t.Fatalf("failed to create indexes: %v", err)
	}

	// Clean up after test
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.Drop(ctx); err != nil {
			t.Logf("warning: failed to drop test database on cleanup: %v", err)
		}
	})

	return db
}

// sanitizeTestName converts a test name to a valid database name suffix.
// MongoDB limits database names to 63 characters, so we truncate if needed.
func sanitizeTestName(name string) string {
	// Replace characters that aren't valid in database names
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	// MongoDB has a 63 character limit for database names.
	// The prefix "stratasave_test_" is 16 characters, so limit suffix to 47.
	const maxLen = 47
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return string(result)
}

// TestContext returns a context with a reasonable timeout for test operations.
func TestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

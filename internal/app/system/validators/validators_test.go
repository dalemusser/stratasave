package validators

import (
	"errors"
	"testing"

	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestEnsureAll(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	err := EnsureAll(ctx, db)
	if err != nil {
		t.Fatalf("EnsureAll() error = %v", err)
	}

	// Verify collections were created
	collections := []string{"users", "pages", "site_settings", "email_verifications", "oauth_states", "audit_logs"}
	for _, coll := range collections {
		exists, err := collectionExists(ctx, db, coll)
		if err != nil {
			t.Errorf("collectionExists(%s) error = %v", coll, err)
			continue
		}
		if !exists {
			t.Errorf("collection %s should exist after EnsureAll", coll)
		}
	}
}

func TestEnsureAll_Idempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Run twice to verify idempotency
	err := EnsureAll(ctx, db)
	if err != nil {
		t.Fatalf("First EnsureAll() error = %v", err)
	}

	err = EnsureAll(ctx, db)
	if err != nil {
		t.Fatalf("Second EnsureAll() error = %v", err)
	}
}

func TestCollectionExists(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Collection doesn't exist
	exists, err := collectionExists(ctx, db, "nonexistent_collection")
	if err != nil {
		t.Fatalf("collectionExists() error = %v", err)
	}
	if exists {
		t.Error("collectionExists() should return false for nonexistent collection")
	}

	// Create collection and verify
	err = db.CreateCollection(ctx, "test_collection")
	if err != nil {
		t.Fatalf("CreateCollection() error = %v", err)
	}

	exists, err = collectionExists(ctx, db, "test_collection")
	if err != nil {
		t.Fatalf("collectionExists() error = %v", err)
	}
	if !exists {
		t.Error("collectionExists() should return true for existing collection")
	}
}

func TestEnsureCollection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// First call should create
	created, err := ensureCollection(ctx, db, "new_collection")
	if err != nil {
		t.Fatalf("First ensureCollection() error = %v", err)
	}
	if !created {
		t.Error("First ensureCollection() should return created=true")
	}

	// Second call should not create
	created, err = ensureCollection(ctx, db, "new_collection")
	if err != nil {
		t.Fatalf("Second ensureCollection() error = %v", err)
	}
	if created {
		t.Error("Second ensureCollection() should return created=false")
	}
}

func TestIsNamespaceExistsErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"generic error", errors.New("some error"), false},
		{"already exists message", errors.New("collection already exists"), true},
		{"namespace exists message", errors.New("namespace exists"), true},
		{"uppercase already exists", errors.New("ALREADY EXISTS"), true},
		{"command error code 48", mongo.CommandError{Code: 48, Message: "exists"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNamespaceExistsErr(tt.err); got != tt.want {
				t.Errorf("isNamespaceExistsErr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNoSuchCommand(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"generic error", errors.New("some error"), false},
		{"no such command message", errors.New("no such command"), true},
		{"uppercase no such command", errors.New("NO SUCH COMMAND"), true},
		{"command error code 59", mongo.CommandError{Code: 59, Message: "command"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoSuchCommand(tt.err); got != tt.want {
				t.Errorf("isNoSuchCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNotImplemented(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"generic error", errors.New("some error"), false},
		{"not implemented message", errors.New("not implemented"), true},
		{"not supported message", errors.New("not supported"), true},
		{"uppercase not implemented", errors.New("NOT IMPLEMENTED"), true},
		{"command error code 115", mongo.CommandError{Code: 115, Message: "impl"}, true},
		{"command error not implemented msg", mongo.CommandError{Code: 0, Message: "not implemented"}, true},
		{"command error not supported msg", mongo.CommandError{Code: 0, Message: "not supported"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotImplemented(tt.err); got != tt.want {
				t.Errorf("isNotImplemented() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUsersSchema(t *testing.T) {
	schema := usersSchema()
	if schema == nil {
		t.Fatal("usersSchema() returned nil")
	}

	jsonSchema, ok := schema["$jsonSchema"]
	if !ok {
		t.Error("usersSchema() should have $jsonSchema key")
		return
	}

	// bson.M is map[string]interface{}
	schemaMap, ok := jsonSchema.(bson.M)
	if !ok {
		t.Errorf("$jsonSchema should be a bson.M, got %T", jsonSchema)
		return
	}

	// Verify required fields
	required, ok := schemaMap["required"]
	if !ok {
		t.Error("schema should have 'required' field")
	}
	if required == nil {
		t.Error("required should not be nil")
	}
}

package savebrowser

import (
	"testing"

	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

func TestStore_ListGames(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	store := NewStore(db, logger)

	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Insert some test data into player_states
	coll := db.Collection(CollectionName)
	docs := []interface{}{
		bson.M{"user_id": "user1", "game": "game_a", "save_data": bson.M{}},
		bson.M{"user_id": "user2", "game": "game_a", "save_data": bson.M{}},
		bson.M{"user_id": "user1", "game": "game_b", "save_data": bson.M{}},
		bson.M{"user_id": "user3", "game": "game_c", "save_data": bson.M{}},
	}
	_, err := coll.InsertMany(ctx, docs)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	games, err := store.ListGames(ctx)
	if err != nil {
		t.Fatalf("ListGames() error = %v", err)
	}

	// Should return distinct game names
	expected := map[string]bool{"game_a": true, "game_b": true, "game_c": true}
	if len(games) != 3 {
		t.Errorf("ListGames() returned %d games, want 3", len(games))
	}
	for _, g := range games {
		if !expected[g] {
			t.Errorf("ListGames() unexpected game %q", g)
		}
	}
}

func TestStore_ListUsers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	store := NewStore(db, logger)

	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Insert test data
	coll := db.Collection(CollectionName)
	docs := []interface{}{
		bson.M{"user_id": "alice", "game": "mygame", "save_data": bson.M{}},
		bson.M{"user_id": "alice", "game": "mygame", "save_data": bson.M{}}, // duplicate user
		bson.M{"user_id": "bob", "game": "mygame", "save_data": bson.M{}},
		bson.M{"user_id": "charlie", "game": "mygame", "save_data": bson.M{}},
		bson.M{"user_id": "alice", "game": "othergame", "save_data": bson.M{}}, // different game
	}
	coll.InsertMany(ctx, docs)

	t.Run("lists distinct users for game", func(t *testing.T) {
		users, hasMore, err := store.ListUsers(ctx, "mygame", "", 10)
		if err != nil {
			t.Fatalf("ListUsers() error = %v", err)
		}
		if hasMore {
			t.Error("ListUsers() hasMore = true, want false")
		}
		if len(users) != 3 {
			t.Errorf("ListUsers() returned %d users, want 3", len(users))
		}
	})

	t.Run("respects limit and returns hasMore", func(t *testing.T) {
		users, hasMore, err := store.ListUsers(ctx, "mygame", "", 2)
		if err != nil {
			t.Fatalf("ListUsers() error = %v", err)
		}
		if !hasMore {
			t.Error("ListUsers() hasMore = false, want true")
		}
		if len(users) != 2 {
			t.Errorf("ListUsers() returned %d users, want 2", len(users))
		}
	})

	t.Run("filters by search prefix", func(t *testing.T) {
		users, _, err := store.ListUsers(ctx, "mygame", "al", 10)
		if err != nil {
			t.Fatalf("ListUsers() error = %v", err)
		}
		if len(users) != 1 || users[0] != "alice" {
			t.Errorf("ListUsers(search='al') = %v, want [alice]", users)
		}
	})
}

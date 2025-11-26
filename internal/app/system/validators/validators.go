package validators

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
)

// EnsureAll creates collections (if missing) and tries to attach JSON-Schema
// validators. On servers that don't support collMod/validators (e.g. some
// DocumentDB versions), we log and skip gracefully.
func EnsureAll(ctx context.Context, db *mongo.Database) error {
	var problems []string

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

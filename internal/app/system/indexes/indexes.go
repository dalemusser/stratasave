// internal/app/system/indexes/indexes.go
package indexes

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
)

/*
EnsureAll is called at startup. Each ensure* function is idempotent.
We aggregate errors so any problem is visible and startup can fail fast.
*/
func EnsureAll(ctx context.Context, db *mongo.Database) error {
	var problems []string

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

/* -------------------------------------------------------------------------- */
/* Core helper: reconcile a set of desired indexes for one collection         */
/* -------------------------------------------------------------------------- */

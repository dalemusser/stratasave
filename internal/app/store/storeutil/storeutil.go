// internal/app/store/storeutil/storeutil.go
package storeutil

import "go.mongodb.org/mongo-driver/mongo/options"

// Paginate returns *options.FindOptions with skip/limit given a 1-based page.
func Paginate(limit, page int64) *options.FindOptions {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	sk := (page - 1) * limit
	return options.Find().SetLimit(limit).SetSkip(sk)
}

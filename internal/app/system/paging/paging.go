// internal/app/system/paging/paging.go
package paging

// PageSize is the default number of rows shown in paged lists.
// Keep this as an int because most call sites add/subtract and then
// cast to int64 for Mongo Find().SetLimit().
const PageSize = 50

// LimitPlusOne returns PageSize+1 as int64 for look‑ahead pagination
// (fetch one extra document to detect hasNext).
func LimitPlusOne() int64 { return int64(PageSize + 1) }

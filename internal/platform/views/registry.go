// internal/platform/views/registry.go
package views

import (
	"io/fs"
	"sync"
)

// Set describes one module's template set.
type Set struct {
	// Name is for logging / debugging only (e.g., "shared", "admin_resources").
	Name string
	// FS is the embedded filesystem from the feature package.
	FS fs.FS
	// Patterns are the glob patterns to load from FS (e.g., []string{"*.gohtml"}).
	Patterns []string
}

var (
	mu       sync.RWMutex
	registry []Set
)

// Register is typically called from a feature package's init().
func Register(s Set) {
	mu.Lock()
	defer mu.Unlock()
	registry = append(registry, s)
}

// All returns the registered template sets.
// The render engine calls this once at boot.
func All() []Set {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Set, len(registry))
	copy(out, registry)
	return out
}

// Reset is handy for tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = nil
}

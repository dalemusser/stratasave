package testutil

import (
	"sync"

	"github.com/dalemusser/stratasave/internal/app/resources"
	"github.com/dalemusser/waffle/pantry/templates"
	"go.uber.org/zap"
)

var bootOnce sync.Once
var bootErr error

// BootTemplatesOnce initializes the template engine for tests.
// It registers shared templates and boots the engine exactly once,
// making subsequent calls safe and efficient.
//
// Call this in tests that invoke handlers which render templates.
// The function is safe to call multiple times; only the first call
// has any effect.
//
// Usage:
//
//	func TestHandler(t *testing.T) {
//	    testutil.BootTemplatesOnce()
//	    // ... rest of test
//	}
//
// Note: Feature templates are registered via init() when the feature
// package is imported, so they're automatically available when testing
// that feature.
func BootTemplatesOnce() error {
	bootOnce.Do(func() {
		// Register shared templates (layout, menu, etc.)
		resources.LoadSharedTemplates()

		// Create and boot the template engine
		eng := templates.New(false)
		logger := zap.NewNop()

		bootErr = eng.Boot(logger)
		if bootErr != nil {
			return
		}

		// Install the engine for package-level Render functions
		templates.UseEngine(eng, logger)
	})
	return bootErr
}

// MustBootTemplates boots templates and fails the test if there's an error.
// This is the recommended way to initialize templates in tests.
//
// Usage:
//
//	func TestHandler(t *testing.T) {
//	    testutil.MustBootTemplates(t)
//	    // ... rest of test
//	}
func MustBootTemplates(t interface{ Fatalf(string, ...any) }) {
	if err := BootTemplatesOnce(); err != nil {
		t.Fatalf("failed to boot templates: %v", err)
	}
}

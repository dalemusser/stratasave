// internal/app/system/viewsollect/register.go
package viewscollect

// Import for side effects: each package’s init() runs and calls views.Register(...)
import (
	_ "github.com/dalemusser/stratalog/internal/app/features/shared/views"
)

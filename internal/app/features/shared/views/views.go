// internal/app/features/shared/views/views.go
package views

import (
	"embed"

	pviews "github.com/dalemusser/stratasave/internal/platform/views"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	pviews.Register(pviews.Set{
		Name:     "shared",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

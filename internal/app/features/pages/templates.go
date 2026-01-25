// internal/app/features/pages/templates.go
package pages

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "pages",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

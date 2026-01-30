// internal/app/features/savebrowser/templates.go
package savebrowser

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "savebrowser",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

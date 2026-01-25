// internal/app/features/status/templates.go
package status

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "status",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

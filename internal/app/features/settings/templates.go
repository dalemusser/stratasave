// internal/app/features/settings/templates.go
package settings

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "settings",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

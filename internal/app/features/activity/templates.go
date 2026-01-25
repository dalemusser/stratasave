// internal/app/features/activity/templates.go
package activity

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "activity",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

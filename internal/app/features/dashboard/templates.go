// internal/app/features/dashboard/templates.go
package dashboard

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "dashboard",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

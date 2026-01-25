// internal/app/features/announcements/templates.go
package announcements

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "announcements",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

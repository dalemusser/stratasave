// internal/app/features/invitations/templates.go
package invitations

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "invitations",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

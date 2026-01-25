// internal/app/features/apikeys/templates.go
package apikeysfeature

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "apikeys",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

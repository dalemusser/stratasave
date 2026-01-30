// internal/app/features/apistats/templates.go
package apistats

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "apistats",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

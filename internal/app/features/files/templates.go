// Package files provides the Files feature with nested folder support.
package files

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "files",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

// internal/app/features/login/templates.go
package login

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "login",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

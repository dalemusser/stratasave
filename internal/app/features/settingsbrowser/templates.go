// internal/app/features/settingsbrowser/templates.go
package settingsbrowser

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "settingsbrowser",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

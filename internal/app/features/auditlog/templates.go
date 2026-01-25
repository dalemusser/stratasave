// internal/app/features/auditlog/templates.go
package auditlog

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "auditlog",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

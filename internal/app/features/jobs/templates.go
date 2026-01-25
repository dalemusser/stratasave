// internal/app/features/jobs/templates.go
package jobsfeature

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "jobs",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

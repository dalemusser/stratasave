// internal/app/features/ledger/templates.go
package ledgerfeature

import (
	"embed"

	"github.com/dalemusser/waffle/pantry/templates"
)

//go:embed templates/*.gohtml
var FS embed.FS

func init() {
	templates.Register(templates.Set{
		Name:     "ledger",
		FS:       FS,
		Patterns: []string{"templates/*.gohtml"},
	})
}

// internal/app/resources/resources.go
package resources

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/dalemusser/waffle/pantry/templates"
)

// Embed shared template files (layout, menu, etc.).
//
//go:embed templates/*.gohtml
var sharedFS embed.FS

//go:embed assets/css/*.css assets/js/*.js
var assetsFS embed.FS

var registerOnce sync.Once

// LoadSharedTemplates registers shared templates (layout, menu) with the waffle template engine.
// This must be called before templates.Boot() in BuildHandler.
func LoadSharedTemplates() {
	registerOnce.Do(func() {
		templates.Register(templates.Set{
			Name:     "shared",
			FS:       sharedFS,
			Patterns: []string{"templates/*.gohtml"},
		})
	})
}

// Assets returns the embedded assets filesystem.
func Assets() fs.FS {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic("failed to get assets subdirectory: " + err.Error())
	}
	return sub
}

// AssetsHandler returns an http.Handler that serves embedded assets.
// The prefix is stripped from the request path before looking up files.
func AssetsHandler(prefix string) http.Handler {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic("failed to get assets subdirectory: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip the prefix from the path
		path := strings.TrimPrefix(r.URL.Path, prefix)
		path = strings.TrimPrefix(path, "/")

		r.URL.Path = "/" + path
		fileServer.ServeHTTP(w, r)
	})
}

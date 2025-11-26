// internal/app/system/server/mw_blockscans.go
package server

import (
	"net/http"
	"strings"
)

var badPrefixes = []string{
	"/wp-", "/wp/", "/xmlrpc.php", "/.well-known/", "/vendor/phpunit",
	"/phpmyadmin", "/.env", "/.git",
}
var badSuffixes = []string{".php", ".php7", ".php8", ".phP", ".bak", ".sql", ".zip"}

func blockScans(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.ToLower(r.URL.Path)
		for _, s := range badSuffixes {
			if strings.HasSuffix(p, s) {
				http.NotFound(w, r)
				return
			}
		}
		for _, pre := range badPrefixes {
			if strings.HasPrefix(p, pre) {
				http.NotFound(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

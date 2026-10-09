// Package web serves the built single-page app (web/dist) with an
// index.html fallback for client-side routes.
package web

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Handler serves dir; unknown paths without a file extension get index.html.
func Handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean))); err != nil && path.Ext(clean) == "" {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		if strings.HasPrefix(clean, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

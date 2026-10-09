// Package web serves the built single-page app (web/dist) with an
// index.html fallback for client-side routes. Every response that serves
// the HTML also sets the session cookie (session.Page), so the page's API
// calls always arrive with it.
package web

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pierre10101/seatlane/internal/session"
)

// Handler serves dir. "/", "/index.html" and unknown paths without a file
// extension get index.html with the session cookie; other files are static.
func Handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean)))
		if clean == "/" || clean == "/index.html" || (err != nil && path.Ext(clean) == "") {
			serveIndex(w, r, index)
			return
		}
		if strings.HasPrefix(clean, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

// serveIndex answers with index.html (no ServeFile: it redirects
// /index.html to /), after setting the session cookie.
func serveIndex(w http.ResponseWriter, r *http.Request, index string) {
	f, err := os.Open(index)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	session.Page(w, r)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", st.ModTime(), f)
}

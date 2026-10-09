package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dist(t *testing.T) string {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"index.html": "<!doctype html><title>Seatlane</title>", "assets/app.js": "console.log(1)", "favicon.svg": "<svg/>"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Every response that serves the HTML (/, /index.html, the SPA fallback)
// runs the page hook once; static files don't.
func TestHTMLRunsThePageHook(t *testing.T) {
	calls := 0
	h := Handler(dist(t), func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.SetCookie(w, &http.Cookie{Name: "page", Value: "1"})
	})
	for _, p := range []string{"/", "/index.html", "/events/2", "/events/2/", "/sign-in"} {
		before := calls
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>Seatlane") || calls != before+1 || len(rec.Result().Cookies()) != 1 {
			t.Fatalf("%s: %d calls %d %q", p, rec.Code, calls-before, rec.Body)
		}
	}
	for _, p := range []string{"/assets/app.js", "/favicon.svg"} {
		before := calls
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK || calls != before || len(rec.Result().Cookies()) != 0 {
			t.Fatalf("%s: %d %v", p, rec.Code, rec.Result().Cookies())
		}
	}
	rec := httptest.NewRecorder()
	Handler(dist(t), nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("nil hook: %d", rec.Code)
	}
}

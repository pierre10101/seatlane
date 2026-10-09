package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierre10101/seatlane/internal/session"
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
// sets exactly one session cookie and keeps a valid one; static files don't.
func TestHTMLAlwaysCarriesTheSessionCookie(t *testing.T) {
	h := Handler(dist(t))
	for _, p := range []string{"/", "/index.html", "/events/2", "/events/2/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		cs := rec.Result().Cookies()
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>Seatlane") || len(cs) != 1 ||
			cs[0].Name != session.Cookie || !session.Valid(cs[0].Value) || !cs[0].HttpOnly || cs[0].SameSite != http.SameSiteLaxMode || cs[0].Path != "/" {
			t.Fatalf("%s: %d %v %q", p, rec.Code, cs, rec.Body)
		}
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req.AddCookie(cs[0])
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if again := rec.Result().Cookies(); len(again) != 1 || again[0].Value != cs[0].Value {
			t.Fatalf("%s: a valid session was replaced: %v", p, again)
		}
	}
	for _, p := range []string{"/assets/app.js", "/favicon.svg"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 0 {
			t.Fatalf("%s: %d %v", p, rec.Code, rec.Result().Cookies())
		}
	}
}

package main

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
	"github.com/pierre10101/seatlane/internal/seed"
)

// A browser that loads the page gets the session cookie with the HTML, so
// its very first API call already has a session; an API call without one
// gets F8 and a cookie (the fallback), and the next call works.
func TestPageCookieCarriesTheFirstAPICall(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "s.db"), seatlane.Schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := seed.IfEmpty(ctx, db); err != nil {
		t.Fatal(err)
	}
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(API(db), dist))
	t.Cleanup(srv.Close)

	do := func(c *http.Client, method, path, body string) *http.Response {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}

	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar}
	if res := do(browser, http.MethodGet, "/events/1", ""); res.StatusCode != 200 || len(res.Cookies()) != 1 {
		t.Fatalf("page: %d %v", res.StatusCode, res.Cookies())
	}
	if res := do(browser, http.MethodPost, "/api/holds", `{"seat_id": 1}`); res.StatusCode != http.StatusCreated || len(res.Cookies()) != 0 {
		t.Fatalf("first hold after the page: %d %v", res.StatusCode, res.Cookies())
	}

	jar2, _ := cookiejar.New(nil)
	direct := &http.Client{Jar: jar2}
	if res := do(direct, http.MethodGet, "/api/events/1/holds", ""); res.StatusCode != http.StatusUnauthorized || len(res.Cookies()) != 1 {
		t.Fatalf("API without a cookie: %d %v", res.StatusCode, res.Cookies())
	}
	if res := do(direct, http.MethodPost, "/api/holds", `{"seat_id": 1}`); res.StatusCode != http.StatusConflict {
		t.Fatalf("second visitor on a held seat: %d", res.StatusCode)
	}
}

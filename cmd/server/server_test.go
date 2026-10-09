package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
	"github.com/pierre10101/seatlane/internal/auth"
	"github.com/pierre10101/seatlane/internal/seed"
	"github.com/pierre10101/seatlane/internal/testkit"
	"golang.org/x/crypto/bcrypt"
)

// The checks serve routes through testkit.AppRoles; it must be this app's
// list.
func TestAppRolesMatchTestkit(t *testing.T) {
	for _, r := range []string{"customer", "organizer", "admin"} {
		if !AppRoles.Has(r) || !testkit.AppRoles.Has(r) {
			t.Fatalf("role %q missing", r)
		}
	}
	for _, r := range []string{"", "organiser", "superuser", "Customer"} {
		if AppRoles.Has(r) || testkit.AppRoles.Has(r) {
			t.Fatalf("role %q declared", r)
		}
	}
}

type browser struct {
	t    *testing.T
	srv  *httptest.Server
	c    *http.Client
	base *url.URL
}

func newServer(t *testing.T) (*httptest.Server, *auth.Service, func(string, ...any)) {
	t.Helper()
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
	accounts := auth.New(db, auth.NewKey())
	accounts.BcryptCost = bcrypt.MinCost
	srv := httptest.NewServer(Handler(API(db, accounts), accounts, dist))
	t.Cleanup(srv.Close)
	exec := func(q string, args ...any) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	return srv, accounts, exec
}

func newBrowser(t *testing.T, srv *httptest.Server) *browser {
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(srv.URL)
	return &browser{t, srv, &http.Client{Jar: jar}, u}
}

func (b *browser) cookie(name string) string {
	for _, c := range b.c.Jar.Cookies(b.base) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// do sends a request; csrf "" sends no X-CSRF-Token header, "jar" sends the
// browser's current seatlane_csrf cookie, anything else that value.
func (b *browser) do(method, path, body, csrf string, header ...string) (int, string, http.Header) {
	b.t.Helper()
	req, _ := http.NewRequest(method, b.srv.URL+path, strings.NewReader(body))
	switch csrf {
	case "":
	case "jar":
		req.Header.Set(auth.CSRFHeader, b.cookie(auth.CSRFCookie))
	default:
		req.Header.Set(auth.CSRFHeader, csrf)
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	res, err := b.c.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(out), res.Header
}

func errorID(body string) string {
	var e httpx.ErrorBody
	_ = json.Unmarshal([]byte(body), &e)
	return e.Error.ID
}

// The whole sign-in flow over real HTTP: CSRF on every state-changing
// request, sign-up signs in, holds belong to the signed-in user, roles are
// enforced, sign-out ends the session on the server.
func TestSignInFlowCSRFAndRoles(t *testing.T) {
	srv, _, exec := newServer(t)
	b := newBrowser(t, srv)

	// The page sets the CSRF cookie; signed out, /api/me is 401.
	if code, _, _ := b.do(http.MethodGet, "/events/1", "", ""); code != 200 || b.cookie(auth.CSRFCookie) == "" {
		t.Fatalf("page: %d, csrf cookie %q", code, b.cookie(auth.CSRFCookie))
	}
	if code, body, _ := b.do(http.MethodGet, "/api/me", "", ""); code != 401 || errorID(body) != "unauthorized" {
		t.Fatalf("me signed out: %d %s", code, body)
	}
	// Seat map is public.
	if code, body, _ := b.do(http.MethodGet, "/api/events/1/seats", "", ""); code != 200 {
		t.Fatalf("seat map signed out: %d %s", code, body)
	}
	// State-changing without the CSRF header: F19, before anything runs.
	if code, body, _ := b.do(http.MethodPost, "/api/sign-up", `{"email":"a@example.com","password":"correct horse"}`, ""); code != 403 || errorID(body) != "F19" {
		t.Fatalf("sign-up without csrf: %d %s", code, body)
	}
	// With CSRF but signed out, a hold is 401 (the runtime, not F-IDs).
	if code, body, _ := b.do(http.MethodPost, "/api/holds", `{"seat_id":1}`, "jar"); code != 401 || errorID(body) != "unauthorized" {
		t.Fatalf("hold signed out: %d %s", code, body)
	}
	preSignIn := b.cookie(auth.CSRFCookie)

	code, body, _ := b.do(http.MethodPost, "/api/sign-up", `{"email":"  Alice@Example.com ","password":"correct horse"}`, "jar")
	var acc auth.Account
	if code != 201 || json.Unmarshal([]byte(body), &acc) != nil || acc.Email != "alice@example.com" || acc.Role != "customer" || acc.UserID < 1 {
		t.Fatalf("sign-up: %d %s", code, body)
	}
	if b.cookie(auth.AuthCookie) == "" || b.cookie(auth.CSRFCookie) == preSignIn {
		t.Fatal("sign-up must set the auth cookie and a new CSRF cookie")
	}
	// The pre-sign-in token is bound to the signed-out browser: F19 now.
	if code, body, _ := b.do(http.MethodPost, "/api/holds", `{"seat_id":1}`, preSignIn); code != 403 || errorID(body) != "F19" {
		t.Fatalf("old csrf token: %d %s", code, body)
	}
	// Another origin, even with the right token: F19.
	if code, body, _ := b.do(http.MethodPost, "/api/holds", `{"seat_id":1}`, "jar", "Origin", "https://evil.example"); code != 403 || errorID(body) != "F19" {
		t.Fatalf("cross-origin: %d %s", code, body)
	}
	if code, body, _ := b.do(http.MethodPost, "/api/holds", `{"seat_id":1}`, "jar", "Sec-Fetch-Site", "cross-site"); code != 403 || errorID(body) != "F19" {
		t.Fatalf("cross-site: %d %s", code, body)
	}
	// Same origin with the token: the hold is the signed-in user's.
	if code, body, _ := b.do(http.MethodPost, "/api/holds", `{"seat_id":1}`, "jar", "Origin", srv.URL); code != 201 {
		t.Fatalf("hold: %d %s", code, body)
	}
	if code, body, _ := b.do(http.MethodPost, "/api/holds", `{"seat_id":2,"user":99}`, "jar"); code != 400 {
		t.Fatalf("caller-sent user: %d %s", code, body)
	}
	if code, body, _ := b.do(http.MethodGet, "/api/me", "", ""); code != 200 || !strings.Contains(body, `"alice@example.com"`) {
		t.Fatalf("me: %d %s", code, body)
	}
	if code, body, _ := b.do(http.MethodGet, "/api/events/1/holds", "", ""); code != 200 || !strings.Contains(body, `"seat_id":1`) {
		t.Fatalf("my holds: %d %s", code, body)
	}

	// An organizer (created by hand: sign-up only makes customers) may not hold.
	org := newBrowser(t, srv)
	hash, _ := bcrypt.GenerateFromPassword([]byte("organizer pass"), bcrypt.MinCost)
	exec(`INSERT INTO users (email, password_hash, role, created_at) VALUES ('olga@example.com', ?, 'organizer', 0)`, string(hash))
	org.do(http.MethodGet, "/", "", "")
	if code, body, _ := org.do(http.MethodPost, "/api/sign-in", `{"email":"olga@example.com","password":"organizer pass"}`, "jar"); code != 200 || !strings.Contains(body, `"organizer"`) {
		t.Fatalf("organizer sign-in: %d %s", code, body)
	}
	if code, body, _ := org.do(http.MethodPost, "/api/holds", `{"seat_id":3}`, "jar"); code != 403 || errorID(body) != "forbidden" {
		t.Fatalf("organizer hold: %d %s", code, body)
	}

	// Sign-out ends the session on the server: the copied token is useless.
	stolen := b.cookie(auth.AuthCookie)
	if code, body, _ := b.do(http.MethodPost, "/api/sign-out", "", "jar"); code != 200 {
		t.Fatalf("sign-out: %d %s", code, body)
	}
	if b.cookie(auth.AuthCookie) != "" {
		t.Fatal("sign-out must expire the auth cookie")
	}
	if code, _, _ := b.do(http.MethodGet, "/api/me", "", ""); code != 401 {
		t.Fatalf("me after sign-out: %d", code)
	}
	replay := newBrowser(t, srv)
	replay.c.Jar.SetCookies(replay.base, []*http.Cookie{{Name: auth.AuthCookie, Value: stolen}})
	if code, _, _ := replay.do(http.MethodGet, "/api/me", "", ""); code != 401 {
		t.Fatalf("replayed token after sign-out: %d", code)
	}
}

// Sign-in answers: unknown email and wrong password are the same F17; the
// sixth attempt from one IP for one email within 900 s is F18 (429 with
// Retry-After) even with the right password; at 900 s it is allowed again.
func TestSignInFailuresAndRateLimitOverHTTP(t *testing.T) {
	srv, _, _ := newServer(t)
	clock := time.Unix(testkit.T0, 0)
	old := httpx.Now
	httpx.Now = func() time.Time { return clock }
	t.Cleanup(func() { httpx.Now = old })

	b := newBrowser(t, srv)
	b.do(http.MethodGet, "/", "", "")
	if code, body, _ := b.do(http.MethodPost, "/api/sign-up", `{"email":"bob@example.com","password":"bobs password"}`, "jar"); code != 201 {
		t.Fatalf("sign-up: %d %s", code, body)
	}
	b.do(http.MethodPost, "/api/sign-out", "", "jar")

	_, wrong, _ := b.do(http.MethodPost, "/api/sign-in", `{"email":"bob@example.com","password":"not bobs password"}`, "jar")
	_, unknown, _ := b.do(http.MethodPost, "/api/sign-in", `{"email":"nobody@example.com","password":"not bobs password"}`, "jar")
	if errorID(wrong) != "F17" || wrong != unknown {
		t.Fatalf("wrong password %s vs unknown email %s", wrong, unknown)
	}
	for i := 2; i <= 5; i++ {
		if code, body, _ := b.do(http.MethodPost, "/api/sign-in", `{"email":"BOB@example.com","password":"guess"}`, "jar"); code != 401 || errorID(body) != "F17" {
			t.Fatalf("failure %d: %d %s", i, code, body)
		}
	}
	clock = clock.Add(899 * time.Second)
	code, body, h := b.do(http.MethodPost, "/api/sign-in", `{"email":"bob@example.com","password":"bobs password"}`, "jar")
	if code != 429 || errorID(body) != "F18" || h.Get("Retry-After") != "1" {
		t.Fatalf("sixth attempt at +899 s: %d %s retry-after %q", code, body, h.Get("Retry-After"))
	}
	clock = clock.Add(1 * time.Second)
	if code, body, _ := b.do(http.MethodPost, "/api/sign-in", `{"email":"bob@example.com","password":"bobs password"}`, "jar"); code != 200 {
		t.Fatalf("at +900 s: %d %s", code, body)
	}
	if code, body, _ := b.do(http.MethodPost, "/api/sign-in", `{"email":"bob@example.com"}`, "jar"); code != 400 || errorID(body) != "bad_request" {
		t.Fatalf("missing password: %d %s", code, body)
	}
}

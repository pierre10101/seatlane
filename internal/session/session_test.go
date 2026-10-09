package session

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A minted session is 128 random bits as 26 base32 characters, text the
// runtime accepts, and never repeats.
func TestMintIsValidText(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		s := Mint()
		if len(s) != 26 || !Valid(s) || seen[s] {
			t.Fatalf("minted %q", s)
		}
		seen[s] = true
	}
	for _, bad := range []string{"", "a b", "abc!", "é", strings.Repeat("x", 129)} {
		if Valid(bad) {
			t.Fatalf("%q valid", bad)
		}
	}
	for _, good := range []string{"0", "abc", "A-b_c.9", strings.Repeat("x", 128)} {
		if !Valid(good) {
			t.Fatalf("%q invalid", good)
		}
	}
}

func cookieOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	c := rec.Result().Cookies()
	if len(c) != 1 {
		t.Fatalf("want exactly one Set-Cookie, got %v", c)
	}
	if c[0].Name != "bridge_session" || !c[0].HttpOnly || c[0].SameSite != http.SameSiteLaxMode || c[0].Path != "/" || c[0].MaxAge <= 0 {
		t.Fatalf("cookie attributes %+v", c[0])
	}
	return c[0]
}

// Issue mints a cookie only when the request has no valid one, and never
// changes the request (no body or query injection).
func TestIssueSetsCookieOnlyWithoutAValidOne(t *testing.T) {
	var gotBody, gotQuery string
	h := Issue(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotQuery = string(b), r.URL.RawQuery
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/holds?limit=5", strings.NewReader(`{"seat_id":3}`)))
	if gotBody != `{"seat_id":3}` || gotQuery != "limit=5" {
		t.Fatalf("request changed: body %s query %s", gotBody, gotQuery)
	}
	if c := cookieOf(t, rec); !Valid(c.Value) || c.Secure {
		t.Fatalf("minted %+v", c)
	}
	for value, mints := range map[string]bool{"k3j2h4g5f6d7s8a9q0w1e2r3t4": false, "abc": false, "": true, "abc!": true, strings.Repeat("x", 129): true} {
		req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
		req.AddCookie(&http.Cookie{Name: Cookie, Value: value})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := len(rec.Result().Cookies()) == 1; got != mints {
			t.Fatalf("cookie %q: minted=%v", value, got)
		}
	}
	// Two bridge_session cookies: the runtime reads no session, so mint.
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.AddCookie(&http.Cookie{Name: Cookie, Value: "aaa"})
	req.AddCookie(&http.Cookie{Name: Cookie, Value: "bbb"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	cookieOf(t, rec)
}

// Page always sets the cookie: it keeps (renews) a valid session, replaces
// a missing or invalid one, and is Secure over TLS.
func TestPageAlwaysSetsTheCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	s := Page(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if c := cookieOf(t, rec); c.Value != s || !Valid(s) || c.Secure {
		t.Fatalf("new: %+v", c)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: Cookie, Value: s})
	rec = httptest.NewRecorder()
	if got := Page(rec, req); got != s || cookieOf(t, rec).Value != s {
		t.Fatalf("renewal changed the session: %q -> %q", s, got)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: Cookie, Value: "abc!"})
	rec = httptest.NewRecorder()
	if got := Page(rec, req); got == "abc!" || !Valid(got) {
		t.Fatalf("invalid kept: %q", got)
	}
	req = httptest.NewRequest(http.MethodGet, "https://seatlane.example/", nil)
	req.TLS = &tls.ConnectionState{}
	rec = httptest.NewRecorder()
	Page(rec, req)
	if !cookieOf(t, rec).Secure {
		t.Fatal("not Secure over TLS")
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec = httptest.NewRecorder()
	Page(rec, req)
	if !cookieOf(t, rec).Secure {
		t.Fatal("not Secure behind a TLS proxy")
	}
}

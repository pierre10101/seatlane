// Package session issues every visitor an anonymous session cookie.
//
// It never touches a request: the actions take the session as a server-set
// input (`server:"session"`, bridge-en grammar T2), which the bridge-en
// runtime's httpx.Bind reads from the cookie httpx.SessionCookie
// ("bridge_session"); without a valid cookie the session is the empty text
// and the action answers F8. A body or query string that sends `session` is
// refused by httpx.Bind with 400. This package only makes sure a browser
// gets that cookie:
//
//   - Page sets it on every response that serves the web app's HTML, so the
//     page's API calls always arrive with it (the existing value is kept and
//     renewed; a missing or invalid one is replaced by a new one);
//   - Issue sets it on any response whose request had no valid one, as a
//     fallback for API calls made without the page (the Vite dev server, a
//     cookie that expired between page load and click).
//
// A session is 128 random bits from crypto/rand, written as 26 lower-case
// base32 characters: text the runtime accepts (1 to 128 of [A-Za-z0-9._-]).
package session

import (
	"crypto/rand"
	"encoding/base32"
	"net/http"
	"strings"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
)

// Cookie is the session cookie's name, which the runtime fixes.
const Cookie = httpx.SessionCookie

// maxAge keeps a session for 30 days.
const maxAge = 60 * 60 * 24 * 30

// maxText is the longest session text the runtime accepts.
const maxText = 128

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// Mint returns a new session: 128 random bits, base32, lower case (26 chars).
func Mint() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return strings.ToLower(encoding.EncodeToString(b[:]))
}

// Valid reports whether s is a session the runtime accepts: 1 to 128
// letters, digits, '-', '_' or '.' (httpx.SessionValue["string"]).
func Valid(s string) bool {
	if s == "" || len(s) > maxText {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

// Current returns the request's session the way httpx.Bind reads it: the
// value of its only bridge_session cookie, if valid; ok is false otherwise
// (no cookie, several, or an invalid value).
func Current(r *http.Request) (string, bool) {
	value, n := "", 0
	for _, c := range r.Cookies() {
		if c.Name == Cookie {
			value, n = c.Value, n+1
		}
	}
	return value, n == 1 && Valid(value)
}

// Page sets the session cookie on w for a response that serves the web
// app's HTML: the request's valid session, renewed, or a new one. It
// returns the session the browser will send from now on.
func Page(w http.ResponseWriter, r *http.Request) string {
	s, ok := Current(r)
	if !ok {
		s = Mint()
	}
	set(w, r, s)
	return s
}

// Issue sets a new session cookie on the response when the request carries
// no valid one. It does not change the request: a request that arrived
// without a cookie is still answered as having no session (F8 from the
// API); the browser sends the cookie from its next request on.
func Issue(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := Current(r); !ok {
			set(w, r, Mint())
		}
		next.ServeHTTP(w, r)
	})
}

// set writes the cookie: HttpOnly, SameSite=Lax, Path=/, and Secure when
// the request came over TLS (directly, or via a proxy that says so).
func set(w http.ResponseWriter, r *http.Request, value string) {
	http.SetCookie(w, &http.Cookie{
		Name: Cookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: overTLS(r),
	})
}

func overTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

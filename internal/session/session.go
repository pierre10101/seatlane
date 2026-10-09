// Package session issues every visitor an anonymous session cookie.
//
// It never touches a request: the actions take the session as a server-set
// input (`server:"session"`, bridge-en grammar T2), which the bridge-en
// runtime's httpx.Bind reads from the cookie httpx.SessionCookie
// ("bridge_session"); without a valid cookie the session is 0 and the
// action answers F8. A body or query string that sends `session` is
// refused by httpx.Bind with 400. This package only makes sure a browser
// gets that cookie: Issue sets it on the response when the request has none.
package session

import (
	"crypto/rand"
	"encoding/binary"
	"net/http"
	"strconv"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
)

// Cookie is the session cookie's name, which the runtime fixes.
const Cookie = httpx.SessionCookie

// maxID keeps ids exact in JavaScript numbers (2^53 - 1).
const maxID = 1<<53 - 1

// Mint returns a new random session id in [1, 2^53-1].
func Mint() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return int64(binary.BigEndian.Uint64(b[:])%maxID) + 1
}

// Parse reads a session id; ok is false for anything but 1..2^53-1 in digits.
func Parse(s string) (int64, bool) {
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n >= 1 && n <= maxID
}

// Issue sets a freshly minted session cookie on the response when the
// request carries no valid one. It does not change the request: a request
// that arrived without a cookie is still answered as having no session (F8
// from the API); the browser sends the cookie from its next request on.
func Issue(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(Cookie); err != nil || !valid(c.Value) {
			http.SetCookie(w, &http.Cookie{
				Name: Cookie, Value: strconv.FormatInt(Mint(), 10), Path: "/",
				HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 30,
			})
		}
		next.ServeHTTP(w, r)
	})
}

func valid(s string) bool { _, ok := Parse(s); return ok }

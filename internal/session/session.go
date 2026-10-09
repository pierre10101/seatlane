// Package session gives every visitor an anonymous session id and hands it
// to the bound actions as their `session` input.
//
// bridge-en has no server-set identity input (only the clock, T1), so this
// is plumbing in front of httpx.Bind, in the spirit of httpx.ClockRule: the
// id comes from the HttpOnly cookie seatlane_sid (minted here when absent),
// is written into the JSON body (POST) or the query string (GET) as
// `session`, and a request that sends `session` itself is refused with 400.
package session

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// Cookie is the name of the session cookie.
const Cookie = "seatlane_sid"

// Field is the input field the actions read the session from.
const Field = "session"

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

// Parse reads a session id; ok is false for anything but 1..2^53-1.
func Parse(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n >= 1 && n <= maxID
}

// Wrap injects the visitor's session into every request to next.
func Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := int64(0), false
		if c, err := r.Cookie(Cookie); err == nil {
			id, ok = Parse(c.Value)
		}
		if !ok {
			id = Mint()
			http.SetCookie(w, &http.Cookie{
				Name: Cookie, Value: strconv.FormatInt(id, 10), Path: "/",
				HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 30,
			})
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			q := r.URL.Query()
			if q.Has(Field) {
				refuse(w)
				return
			}
			q.Set(Field, strconv.FormatInt(id, 10))
			r.URL.RawQuery = q.Encode()
		default:
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if err != nil {
				refuse(w)
				return
			}
			var obj map[string]json.RawMessage
			if json.Unmarshal(body, &obj) == nil && obj != nil {
				if _, sent := obj[Field]; sent {
					refuse(w)
					return
				}
				obj[Field] = json.RawMessage(strconv.FormatInt(id, 10))
				body, _ = json.Marshal(obj)
			}
			// Anything that is not one JSON object is passed on unchanged:
			// httpx.Bind answers it with its own 400.
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
		}
		next.ServeHTTP(w, r)
	})
}

func refuse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`{"error":{"id":"bad_request","message":"field \"session\" is set by the server from the session cookie; do not send it"}}` + "\n"))
}

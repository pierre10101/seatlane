package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
)

// CSRF protection (F19, csrf/intent.md). The token is "<nonce>.<mac>":
// nonce is 128 random bits, mac = HMAC-SHA256(key, nonce | SHA-256 of the
// current seatlane_auth cookie value, or "" when signed out). It lives in
// the seatlane_csrf cookie, which the page's script reads and sends back in
// X-CSRF-Token. A hostile page can neither read the cookie nor make a valid
// token for this browser's sign-in without the key.

// binding is what a CSRF token is bound to: the browser's current sign-in.
func binding(authToken string) string {
	if authToken == "" {
		return ""
	}
	return tokenHash(authToken)
}

func (s *Service) mac(nonce, bind string) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(nonce))
	m.Write([]byte{'|'})
	m.Write([]byte(bind))
	return hex.EncodeToString(m.Sum(nil))
}

// newCSRFToken makes a token bound to authToken ("" when signed out).
func (s *Service) newCSRFToken(authToken string) string {
	nonce := randomText(16)
	return nonce + "." + s.mac(nonce, binding(authToken))
}

// validCSRFToken reports whether tok was made by this server for authToken.
func (s *Service) validCSRFToken(tok, authToken string) bool {
	nonce, mac, ok := strings.Cut(tok, ".")
	if !ok || nonce == "" || mac == "" {
		return false
	}
	return hmac.Equal([]byte(mac), []byte(s.mac(nonce, binding(authToken))))
}

// cookieValue returns the value of the request's only cookie named name.
func cookieValue(r *http.Request, name string) (string, bool) {
	value, n := "", 0
	for _, c := range r.Cookies() {
		if c.Name == name {
			value, n = c.Value, n+1
		}
	}
	return value, n == 1 && value != ""
}

// authToken is the request's seatlane_auth cookie value, or "".
func authToken(r *http.Request) string {
	v, _ := cookieValue(r, AuthCookie)
	return v
}

// SetCSRFCookie sets a new CSRF cookie bound to authToken.
func (s *Service) SetCSRFCookie(w http.ResponseWriter, r *http.Request, authToken string) {
	http.SetCookie(w, &http.Cookie{
		Name: CSRFCookie, Value: s.newCSRFToken(authToken), Path: "/", MaxAge: int(SessionTTL),
		HttpOnly: false, // the page's script reads it and echoes it in X-CSRF-Token
		SameSite: http.SameSiteLaxMode, Secure: overTLS(r),
	})
}

// EnsureCSRFCookie sets a CSRF cookie unless the request already has a valid
// one for its current sign-in. The web handler calls it when it serves the
// HTML.
func (s *Service) EnsureCSRFCookie(w http.ResponseWriter, r *http.Request) {
	if tok, ok := cookieValue(r, CSRFCookie); ok && s.validCSRFToken(tok, authToken(r)) {
		return
	}
	s.SetCSRFCookie(w, r, authToken(r))
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// sameOrigin is check 1: an Origin header names this host, and
// Sec-Fetch-Site (when the browser sends it) says same-origin or none.
func sameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	return true
}

// CSRFOK reports whether a state-changing request passes the three checks.
func (s *Service) CSRFOK(r *http.Request) bool {
	if !sameOrigin(r) {
		return false
	}
	cookie, ok := cookieValue(r, CSRFCookie)
	header := r.Header.Get(CSRFHeader)
	if !ok || header == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
		return false
	}
	return s.validCSRFToken(cookie, authToken(r))
}

// CSRF wraps the API: safe methods pass (and get a CSRF cookie if they lack
// a valid one); any other method passes only with a valid token, else it is
// answered F19 with a fresh cookie and never reaches next.
func (s *Service) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if safeMethod(r.Method) {
			s.EnsureCSRFCookie(w, r)
			next.ServeHTTP(w, r)
			return
		}
		if !s.CSRFOK(r) {
			s.SetCSRFCookie(w, r, authToken(r))
			writeFailure(w, F19)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// overTLS: the request came over TLS, directly or via a proxy that says so.
func overTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

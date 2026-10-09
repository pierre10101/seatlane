package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
	"golang.org/x/crypto/bcrypt"
)

const t0 int64 = 1_800_000_000

func newService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	conn, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "auth.db"), seatlane.Schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	s := New(conn, NewKey())
	s.BcryptCost = bcrypt.MinCost
	return s, conn
}

func count(t *testing.T, conn *sql.DB, q string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := conn.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSignUpStoresABcryptHashAndSignsIn(t *testing.T) {
	s, conn := newService(t)
	acc, token, err := s.SignUp(" Alice@Example.COM ", "correct horse", t0)
	if err != nil || acc.Email != "alice@example.com" || acc.Role != RoleCustomer || token == "" {
		t.Fatalf("acc %+v token %q err %v", acc, token, err)
	}
	var hash string
	if err := conn.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, acc.UserID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse" || bcrypt.CompareHashAndPassword([]byte(hash), []byte("correct horse")) != nil {
		t.Fatal("password not stored as a bcrypt hash")
	}
	if count(t, conn, `SELECT COUNT(*) FROM auth_sessions WHERE token_hash = ? AND user_id = ? AND expires_at = ?`, tokenHash(token), acc.UserID, t0+SessionTTL) != 1 ||
		count(t, conn, `SELECT COUNT(*) FROM auth_sessions WHERE token_hash = ?`, token) != 0 {
		t.Fatal("the session must be stored by its token hash only")
	}
}

// F14: the email boundaries; nothing is written.
func TestF14_EmailNotValid(t *testing.T) {
	s, conn := newService(t)
	long := strings.Repeat("a", 254-len("@example.com")) + "@example.com" // 254: allowed
	for _, e := range []string{"", "   ", "alice", "@example.com", "alice@", "a@b@example.com", "alice@example", "al ice@example.com", "alice@exa\tmple.com", long + "x"} {
		if _, _, err := s.SignUp(e, "correct horse", t0); !errors.Is(err, F14) {
			t.Fatalf("%q: want F14, got %v", e, err)
		}
	}
	if n := count(t, conn, `SELECT COUNT(*) FROM users`); n != 0 {
		t.Fatalf("%d users written", n)
	}
	if _, _, err := s.SignUp(long, "correct horse", t0); err != nil {
		t.Fatalf("254 characters is allowed: %v", err)
	}
}

// F15: 9 and 73 bytes are refused, 10 and 72 allowed; nothing is written.
func TestF15_PasswordLength(t *testing.T) {
	s, conn := newService(t)
	for _, p := range []string{"", strings.Repeat("p", 9), strings.Repeat("p", 73)} {
		if _, _, err := s.SignUp("a@example.com", p, t0); !errors.Is(err, F15) {
			t.Fatalf("%d bytes: want F15, got %v", len(p), err)
		}
	}
	if n := count(t, conn, `SELECT COUNT(*) FROM users`); n != 0 {
		t.Fatalf("%d users written", n)
	}
	for i, p := range []string{strings.Repeat("p", 10), strings.Repeat("p", 72)} {
		if _, _, err := s.SignUp(strconv.Itoa(i)+"@example.com", p, t0); err != nil {
			t.Fatalf("%d bytes: %v", len(p), err)
		}
	}
}

// F16: a duplicate email (also in another letter case) changes nothing,
// and of many parallel sign-ups with one email exactly one wins.
func TestF16_DuplicateEmail(t *testing.T) {
	s, conn := newService(t)
	first, _, err := s.SignUp("dup@example.com", "first password", t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SignUp(" DUP@example.com", "second password", t0+1); !errors.Is(err, F16) {
		t.Fatalf("want F16, got %v", err)
	}
	if _, _, _, err := s.SignIn("1.2.3.4", "dup@example.com", "first password", t0+2); err != nil {
		t.Fatalf("the first account must be unchanged: %v", err)
	}
	if count(t, conn, `SELECT COUNT(*) FROM users`) != 1 || count(t, conn, `SELECT COUNT(*) FROM auth_sessions WHERE user_id = ?`, first.UserID) != 2 {
		t.Fatal("F16 wrote something")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, f16 := 0, 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.SignUp("race@example.com", "race password", t0)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, F16):
				f16++
			default:
				t.Errorf("unexpected %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 || f16 != 7 {
		t.Fatalf("wins %d F16 %d", wins, f16)
	}
}

// F17: wrong password and unknown email are the same failure, create no
// session, and count toward the rate limit.
func TestF17_WrongPasswordOrUnknownEmail(t *testing.T) {
	s, conn := newService(t)
	if _, _, err := s.SignUp("carol@example.com", "carols password", t0); err != nil {
		t.Fatal(err)
	}
	before := count(t, conn, `SELECT COUNT(*) FROM auth_sessions`)
	_, _, _, wrong := s.SignIn("10.0.0.1", "carol@example.com", "Carols password", t0)
	_, _, _, unknown := s.SignIn("10.0.0.1", "nobody@example.com", "carols password", t0)
	if wrong != F17 || unknown != F17 {
		t.Fatalf("wrong %v unknown %v: want the same F17", wrong, unknown)
	}
	if count(t, conn, `SELECT COUNT(*) FROM auth_sessions`) != before {
		t.Fatal("F17 created a session")
	}
	for _, key := range []string{"10.0.0.1|carol@example.com", "10.0.0.1|nobody@example.com"} {
		if count(t, conn, `SELECT failures FROM sign_in_attempts WHERE key = ?`, key) != 1 {
			t.Fatalf("%s: not counted", key)
		}
	}
	acc, token, _, err := s.SignIn("10.0.0.1", "CAROL@example.com ", "carols password", t0+1)
	if err != nil || acc.Email != "carol@example.com" || token == "" {
		t.Fatalf("right password: %+v %v", acc, err)
	}
	if count(t, conn, `SELECT COUNT(*) FROM sign_in_attempts WHERE key = ?`, "10.0.0.1|carol@example.com") != 0 {
		t.Fatal("a successful sign-in clears the count")
	}
}

// F18: 5 failures for one IP+email in the window; the 6th attempt, even
// with the right password, is refused until window_start + 900 (at +899 still
// refused, at +900 allowed). Other IPs and other emails are not limited, and
// parallel guesses cannot pass the limit.
func TestF18_RateLimited(t *testing.T) {
	s, conn := newService(t)
	if _, _, err := s.SignUp("dan@example.com", "dans password", t0); err != nil {
		t.Fatal(err)
	}
	for i := int64(0); i < 5; i++ {
		if _, _, _, err := s.SignIn("10.0.0.2", "dan@example.com", "guess", t0+i); !errors.Is(err, F17) {
			t.Fatalf("failure %d: %v", i+1, err)
		}
	}
	sessions := count(t, conn, `SELECT COUNT(*) FROM auth_sessions`)
	_, _, retry, err := s.SignIn("10.0.0.2", "dan@example.com", "dans password", t0+899)
	if !errors.Is(err, F18) || retry != 1 {
		t.Fatalf("at +899: %v retry %d", err, retry)
	}
	if count(t, conn, `SELECT failures FROM sign_in_attempts WHERE key = '10.0.0.2|dan@example.com'`) != 5 ||
		count(t, conn, `SELECT COUNT(*) FROM auth_sessions`) != sessions {
		t.Fatal("F18 must not raise the count or create a session")
	}
	if _, _, _, err := s.SignIn("10.0.0.3", "dan@example.com", "dans password", t0+10); err != nil {
		t.Fatalf("another IP: %v", err)
	}
	if _, _, _, err := s.SignIn("10.0.0.2", "other@example.com", "guess", t0+10); !errors.Is(err, F17) {
		t.Fatalf("another email from the same IP: %v", err)
	}
	if _, _, _, err := s.SignIn("10.0.0.2", "dan@example.com", "dans password", t0+900); err != nil {
		t.Fatalf("at +900: %v", err)
	}

	// Parallel guesses on a fresh key: at most 5 reach the password check.
	var wg sync.WaitGroup
	var mu sync.Mutex
	f17, f18 := 0, 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _, err := s.SignIn("10.0.0.9", "dan@example.com", "guess", t0+1000)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case errors.Is(err, F17):
				f17++
			case errors.Is(err, F18):
				f18++
			default:
				t.Errorf("unexpected %v", err)
			}
		}()
	}
	wg.Wait()
	if f17 != 5 || f18 != 7 {
		t.Fatalf("parallel: F17 %d F18 %d", f17, f18)
	}
}

// F19: a POST passes the CSRF middleware only with a header equal to the
// cookie, a token this server made for the current sign-in, and no foreign
// Origin or cross-site fetch; otherwise 403 F19, a fresh cookie, and the
// handler never runs. GET passes and gets a cookie.
func TestF19_CSRF(t *testing.T) {
	s, _ := newService(t)
	other, _ := newService(t)
	reached := 0
	h := s.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached++ }))
	send := func(method string, cookie, header string, set func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://seatlane.test/api/holds", nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: cookie})
		}
		if header != "" {
			req.Header.Set(CSRFHeader, header)
		}
		if set != nil {
			set(req)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	rec := send(http.MethodGet, "", "", nil)
	if reached != 1 || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("GET: reached %d cookies %v", reached, rec.Result().Cookies())
	}
	good := rec.Result().Cookies()[0].Value
	signedIn := s.newCSRFToken("some-session-token")
	withAuth := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: AuthCookie, Value: "another-session-token"}) }
	for name, c := range map[string]struct {
		cookie, header string
		set            func(*http.Request)
	}{
		"no header":                   {good, "", nil},
		"no cookie":                   {"", good, nil},
		"header differs":              {good, s.newCSRFToken(""), nil},
		"forged mac":                  {"abc.def", "abc.def", nil},
		"another server's key":        {other.newCSRFToken(""), other.newCSRFToken(""), nil},
		"bound to another sign-in":    {signedIn, signedIn, withAuth},
		"signed-out token, signed in": {good, good, withAuth},
		"foreign origin":              {good, good, func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }},
		"cross-site fetch":            {good, good, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }},
		"same-site subdomain":         {good, good, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-site") }},
	} {
		before := reached
		rec := send(http.MethodPost, c.cookie, c.header, c.set)
		if rec.Code != F19.Status || !strings.Contains(rec.Body.String(), `"F19"`) || reached != before || len(rec.Result().Cookies()) != 1 {
			t.Fatalf("%s: %d %s reached %d", name, rec.Code, rec.Body, reached-before)
		}
	}
	for name, set := range map[string]func(*http.Request){
		"no origin": nil,
		"same origin": func(r *http.Request) {
			r.Header.Set("Origin", "http://seatlane.test")
			r.Header.Set("Sec-Fetch-Site", "same-origin")
		},
	} {
		before := reached
		if rec := send(http.MethodPost, good, good, set); reached != before+1 || rec.Code != 200 {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	tok := s.newCSRFToken("another-session-token")
	if rec := send(http.MethodPost, tok, tok, withAuth); rec.Code != 200 {
		t.Fatalf("token bound to the current sign-in: %d", rec.Code)
	}
}

// The identity hook: a valid session gives the user id and role; an
// expired (expires_at exactly now), deleted or unknown token is nobody.
func TestIdentityHook(t *testing.T) {
	s, conn := newService(t)
	acc, token, err := s.SignUp("erin@example.com", "erins password", t0)
	if err != nil {
		t.Fatal(err)
	}
	req := func(tok string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		if tok != "" {
			r.AddCookie(&http.Cookie{Name: AuthCookie, Value: tok})
		}
		return r
	}
	if u, role, ok := s.Identify(req(token)); !ok || u != strconv.FormatInt(acc.UserID, 10) || role != "customer" {
		t.Fatalf("valid: %q %q %v", u, role, ok)
	}
	for _, tok := range []string{"", "unknown", tokenHash(token)} {
		if _, _, ok := s.Identify(req(tok)); ok {
			t.Fatalf("%q signed in", tok)
		}
	}
	if _, err := conn.Exec(`UPDATE auth_sessions SET expires_at = ?`, now()); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := s.Identify(req(token)); ok {
		t.Fatal("a session whose expires_at is exactly now has expired")
	}
	if _, err := conn.Exec(`UPDATE auth_sessions SET expires_at = ?`, now()+60); err != nil {
		t.Fatal(err)
	}
	if err := s.SignOut(token); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := s.Identify(req(token)); ok {
		t.Fatal("signed out, still signed in")
	}
}

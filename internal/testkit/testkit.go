// Package testkit is shared setup for the slices' checks: a real SQLite
// database with the app schema, a small seat grid, and row inspection.
package testkit

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
)

// T0 is any time: checks pass the current time in (T1).
const T0 int64 = 1_800_000_000

// Signed-in users across checks: account ids, as the server sets the
// `user` input (`server:"user"`). Alice and Bob are customers; Olga is an
// organizer and Ada an admin (for the 403 checks). 0 is nobody.
const (
	Alice int64 = 1
	Bob   int64 = 2
	Olga  int64 = 3
	Ada   int64 = 4
)

// AppRoles mirrors cmd/server's AppRoles for checks that serve a route
// through httpx.Identify (cmd/server's TestAppRolesMatchTestkit keeps the two
// lists equal).
var AppRoles = httpx.AppRoles("customer", "organizer", "admin")

// AsHeader carries the test sign-in: "<user id>:<role>".
const AsHeader = "X-Test-As"

// Serve puts h behind httpx.Identify with a test sign-in hook that reads
// AsHeader, the way cmd/server puts the API behind the app's real hook.
// Requests without the header are not signed in.
func Serve(h http.Handler) http.Handler {
	return httpx.Identify(AppRoles, func(r *http.Request) (string, string, bool) {
		user, role, ok := strings.Cut(r.Header.Get(AsHeader), ":")
		return user, role, ok
	}, h)
}

// As signs req in as user with role ("" user: not signed in).
func As(req *http.Request, user, role string) *http.Request {
	if user != "" {
		req.Header.Set(AsHeader, user+":"+role)
	}
	return req
}

// Open returns a fresh file-backed database (in t.TempDir) with one event
// (id 1) and seats 1..n, all free, priced 45000 cents.
func Open(t *testing.T, n int) *sql.DB {
	t.Helper()
	conn, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "check.db"), seatlane.Schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	Exec(t, conn, `INSERT INTO events (id, name, venue, city, starts_at, tagline, from_price_cents, to_price_cents, currency)
		VALUES (1, 'Check Night', 'Check Hall', 'Cape Town', ?, 'A test event.', 45000, 45000, 'ZAR')`, T0+86400)
	for i := 1; i <= n; i++ {
		Exec(t, conn, `INSERT INTO seats (id, event_id, section, section_rank, row_label, seat_number, price_cents)
			VALUES (?, 1, 'Stalls', 1, 'A', ?, 45000)`, i, i)
	}
	return conn
}

// Exec runs one statement or fails the test.
func Exec(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

// Row is a seat's stored state. HeldBy and SoldTo are 0 for nobody.
type Row struct {
	HeldBy            int64
	HeldAt, ExpiresAt int64
	SoldTo            int64
	SoldAt            int64
}

// Seat reads a seat's stored state.
func Seat(t *testing.T, conn *sql.DB, id int64) Row {
	t.Helper()
	var r Row
	if err := conn.QueryRow(`SELECT held_by, held_at, expires_at, sold_to, sold_at FROM seats WHERE id = ?`, id).
		Scan(&r.HeldBy, &r.HeldAt, &r.ExpiresAt, &r.SoldTo, &r.SoldAt); err != nil {
		t.Fatal(err)
	}
	return r
}

// Do sends one request to h, signed in as user with role ("" user: not
// signed in), and returns the answer.
func Do(h http.Handler, method, target, body, user, role string) *httptest.ResponseRecorder {
	req := As(httptest.NewRequest(method, target, strings.NewReader(body)), user, role)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ErrorID is the answer's error.id ("" when it is not an error answer).
func ErrorID(rec *httptest.ResponseRecorder) string {
	var body httpx.ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Error.ID
}

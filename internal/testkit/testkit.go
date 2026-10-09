// Package testkit is shared setup for the slices' checks: a real SQLite
// database with the app schema, a small seat grid, and row inspection.
package testkit

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
)

// T0 is any time: checks pass the current time in (T1).
const T0 int64 = 1_800_000_000

// Sessions used across checks: text ids like the ones session.Mint issues
// (the bridge_session cookie). The empty text is no session (F8).
const (
	Alice = "alice7qkz2m4xw3vb6r5nd0hjy"
	Bob   = "bob8tcl1pf6es9ga2wu4ki3oxq"
)

// Open returns a fresh file-backed database (in t.TempDir) with one event
// (id 1) and seats 1..n, all free, priced 45000 cents.
func Open(t *testing.T, n int) *sql.DB {
	t.Helper()
	conn, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "check.db"), seatlane.Schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	Exec(t, conn, `INSERT INTO events (id, name, venue, city, starts_at, tagline, from_price_cents, currency)
		VALUES (1, 'Check Night', 'Check Hall', 'Cape Town', ?, 'A test event.', 45000, 'ZAR')`, T0+86400)
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

// Row is a seat's stored state. HeldBy and SoldTo are ” for nobody.
type Row struct {
	HeldBy            string
	HeldAt, ExpiresAt int64
	SoldTo            string
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

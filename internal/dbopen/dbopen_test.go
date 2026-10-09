package dbopen

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
)

// prePhase1 is schema.sql before phase 1 (a02ad81): text session ids in
// held_by/sold_to and no users, auth_sessions or sign_in_attempts tables.
const prePhase1 = `
CREATE TABLE events (
    id INTEGER PRIMARY KEY, name TEXT NOT NULL, venue TEXT NOT NULL, city TEXT NOT NULL,
    starts_at INTEGER NOT NULL, tagline TEXT NOT NULL,
    from_price_cents INTEGER NOT NULL CHECK (from_price_cents > 0),
    to_price_cents INTEGER NOT NULL CHECK (to_price_cents >= from_price_cents),
    currency TEXT NOT NULL, on_sale INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE seats (
    id INTEGER PRIMARY KEY, event_id INTEGER NOT NULL REFERENCES events (id),
    section TEXT NOT NULL, section_rank INTEGER NOT NULL, row_label TEXT NOT NULL,
    seat_number INTEGER NOT NULL, price_cents INTEGER NOT NULL CHECK (price_cents > 0),
    held_by TEXT NOT NULL DEFAULT '', held_at INTEGER NOT NULL DEFAULT 0,
    expires_at INTEGER NOT NULL DEFAULT 0, sold_to TEXT NOT NULL DEFAULT '',
    sold_at INTEGER NOT NULL DEFAULT 0, UNIQUE (event_id, section, row_label, seat_number)
);
INSERT INTO events VALUES (1, 'Show', 'Hall', 'Cape Town', 1, 'x', 100, 100, 'ZAR', 1);
INSERT INTO seats (event_id, section, section_rank, row_label, seat_number, price_cents) VALUES (1, 'Stalls', 1, 'A', 1, 100);
`

func makeDB(t *testing.T, schema string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seatlane.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return path
}

func version(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func outdated(t *testing.T, err error) *OutdatedError {
	t.Helper()
	var out *OutdatedError
	if !errors.As(err, &out) {
		t.Fatalf("want *OutdatedError, got %v", err)
	}
	return out
}

// The owner's case: a seatlane.db from before phase 1 is refused at startup
// with the delete-and-restart message, and the file is left as it was.
func TestPrePhase1DatabaseIsRefused(t *testing.T) {
	path := makeDB(t, prePhase1)
	before, _ := os.ReadFile(path)
	db, err := Open(context.Background(), path)
	if db != nil {
		db.Close()
		t.Fatal("an out-of-date database was opened")
	}
	out := outdated(t, err)
	want := fmt.Sprintf("%s was created by an older version; delete it (rm %s) and restart", path, path)
	if err.Error() != want {
		t.Fatalf("message:\n got %q\nwant %q", err.Error(), want)
	}
	if out.Reason != "seats.held_by is TEXT, want INTEGER" {
		t.Fatalf("reason %q", out.Reason)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("Open changed the out-of-date file")
	}
	if version(t, path) != 0 {
		t.Fatal("an out-of-date file was stamped")
	}
}

func TestMessageForDefaultName(t *testing.T) {
	err := &OutdatedError{Path: "seatlane.db", Reason: "x"}
	if got := err.Error(); got != "seatlane.db was created by an older version; delete it (rm seatlane.db) and restart" {
		t.Fatal(got)
	}
}

func TestNewDatabaseIsCreatedAndStamped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seatlane.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users (email, password_hash, role, created_at) VALUES ('a@b.co', 'h', 'customer', 1)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if v := version(t, path); v != SchemaVersion {
		t.Fatalf("user_version %d, want %d", v, SchemaVersion)
	}
	// Reopening the server's own file works and keeps the data.
	db, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("users %d, %v", n, err)
	}
}

func TestEmptyFileIsNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seatlane.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
}

// A database phase 1 created before this check existed (current tables,
// user_version 0) is accepted and stamped.
func TestUnstampedCurrentDatabaseIsAccepted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seatlane.db")
	db, err := store.Open(context.Background(), path, seatlane.Schema)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if v := version(t, path); v != SchemaVersion {
		t.Fatalf("user_version %d", v)
	}
}

func TestOtherSchemaVersionsAreRefused(t *testing.T) {
	for _, tc := range []struct {
		v     int
		newer bool
	}{{SchemaVersion + 1, true}, {SchemaVersion + 7, true}} {
		path := makeDB(t, seatlane.Schema+fmt.Sprintf("\nPRAGMA user_version = %d;", tc.v))
		_, err := Open(context.Background(), path)
		out := outdated(t, err)
		if out.Newer != tc.newer || !strings.Contains(err.Error(), "a newer version") {
			t.Fatalf("version %d: %v (newer=%v)", tc.v, err, out.Newer)
		}
	}
	if SchemaVersion > 1 {
		path := makeDB(t, seatlane.Schema+"\nPRAGMA user_version = 1;")
		if out := outdated(t, Check(context.Background(), path)); out.Newer {
			t.Fatal("an older stamp reported as newer")
		}
	}
}

func TestColumnAndTableDifferences(t *testing.T) {
	for _, tc := range []struct{ name, schema, reason string }{
		{"missing table", strings.Replace(seatlane.Schema, "CREATE TABLE IF NOT EXISTS sign_in_attempts", "CREATE TABLE IF NOT EXISTS other_attempts", 1), "no sign_in_attempts table"},
		{"missing column", strings.Replace(seatlane.Schema, "    created_at    INTEGER NOT NULL\n", "    created_on    INTEGER NOT NULL\n", 1), "no users.created_at column"},
		{"nullable column", strings.Replace(seatlane.Schema, "sold_at      INTEGER NOT NULL DEFAULT 0", "sold_at      INTEGER", 1), "seats.sold_at has other constraints"},
		{"extra column", strings.Replace(seatlane.Schema, "    created_at    INTEGER NOT NULL\n", "    created_at    INTEGER NOT NULL,\n    nick TEXT\n", 1), "users has 6 columns, want 5"},
	} {
		if tc.schema == seatlane.Schema {
			t.Fatalf("%s: replacement did not apply", tc.name)
		}
		out := outdated(t, Check(context.Background(), makeDB(t, tc.schema)))
		if out.Reason != tc.reason {
			t.Fatalf("%s: reason %q, want %q", tc.name, out.Reason, tc.reason)
		}
	}
}

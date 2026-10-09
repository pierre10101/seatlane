package main

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// runMain runs this test binary as the server (main with args) and returns
// its exit code, stdout and stderr.
func runMain(t *testing.T, env []string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperMain$")
	cmd.Env = append(append(os.Environ(), "SEATLANE_HELPER_MAIN="+strings.Join(args, "\x1f")), env...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, out.String(), errb.String()
}

func TestHelperMain(t *testing.T) {
	args := os.Getenv("SEATLANE_HELPER_MAIN")
	if args == "" {
		t.Skip("helper process for runMain")
	}
	os.Args = append([]string{"seatlane"}, strings.Split(args, "\x1f")...)
	main()
	os.Exit(0)
}

// A seatlane.db from before phase 1 stops the server at startup with the
// delete-and-restart message (exit 1), instead of a scan error per request.
func TestStartupRefusesPrePhase1Database(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seatlane.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE events (id INTEGER PRIMARY KEY, name TEXT NOT NULL, venue TEXT NOT NULL, city TEXT NOT NULL,
		starts_at INTEGER NOT NULL, tagline TEXT NOT NULL, from_price_cents INTEGER NOT NULL, to_price_cents INTEGER NOT NULL,
		currency TEXT NOT NULL, on_sale INTEGER NOT NULL DEFAULT 1);
	CREATE TABLE seats (id INTEGER PRIMARY KEY, event_id INTEGER NOT NULL REFERENCES events (id), section TEXT NOT NULL,
		section_rank INTEGER NOT NULL, row_label TEXT NOT NULL, seat_number INTEGER NOT NULL, price_cents INTEGER NOT NULL,
		held_by TEXT NOT NULL DEFAULT '', held_at INTEGER NOT NULL DEFAULT 0, expires_at INTEGER NOT NULL DEFAULT 0,
		sold_to TEXT NOT NULL DEFAULT '', sold_at INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	code, _, stderr := runMain(t, nil, "-db", path, "-web", "", "-addr", "127.0.0.1:0")
	want := path + " was created by an older version; delete it (rm " + path + ") and restart"
	if code != 1 || !strings.Contains(stderr, want) || !strings.Contains(stderr, "seats.held_by is TEXT, want INTEGER") {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
}

func TestSeedDevAccountsNeedsDevEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seatlane.db")
	code, _, stderr := runMain(t, []string{"SEATLANE_DEV="}, "-db", path, "-seed-dev-accounts")
	if code != 2 || !strings.Contains(stderr, "refusing to create dev accounts: set SEATLANE_DEV=1") {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the refused seed created the database")
	}
	code, stdout, stderr := runMain(t, []string{"SEATLANE_DEV=1", "SEATLANE_DEV_PASSWORD=dev-password-123"}, "-db", path, "-seed-dev-accounts")
	if code != 0 || !strings.Contains(stdout, "organizer@example.test") || !strings.Contains(stdout, "admin@example.test") || !strings.Contains(stdout, "dev-password-123") {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

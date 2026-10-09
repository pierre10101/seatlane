// Package dbopen opens the app's SQLite file for the server, refusing one
// whose schema an older (or newer) Seatlane created.
//
// store.Open applies schema.sql with CREATE TABLE IF NOT EXISTS, which
// never changes a table that already exists. A database from before
// phase 1 (seats.held_by TEXT, no users table) would open fine and then fail
// on every request (sql: Scan error on column "held_by"). There are no
// migrations: Open detects the mismatch at startup and the server exits with
// a message saying to delete the file.
//
// Two checks, both before the schema is applied:
//
//   - PRAGMA user_version: Open stamps SchemaVersion on every database it
//     opens. A file with another non-zero version was made by another
//     version of the app.
//   - The tables and columns: every table in schema.sql must exist in the
//     file with the same columns (name, declared type, NOT NULL, primary
//     key), compared with schema.sql applied to an in-memory database. This
//     catches files from before user_version was stamped (version 0) and a
//     schema.sql changed without bumping SchemaVersion.
//
// A missing or empty file is new: the schema is applied and stamped.
package dbopen

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
	_ "modernc.org/sqlite" // the "sqlite" driver (store uses the same one)
)

// SchemaVersion is the PRAGMA user_version of a database whose tables are
// schema.sql's. Bump it whenever schema.sql changes a table that already
// exists; databases stamped with an older version are then refused.
//
//	1: phase 1 (users, auth_sessions, sign_in_attempts; held_by/sold_to INTEGER)
const SchemaVersion = 1

// OutdatedError says the file was created by another version of the app.
type OutdatedError struct {
	Path   string
	Reason string // what differs, e.g. "seats.held_by is TEXT, want INTEGER"
	Newer  bool   // the file's user_version is above SchemaVersion
}

// Error is the message the server exits with, e.g. "seatlane.db was
// created by an older version; delete it (rm seatlane.db) and restart".
// Reason has the detail.
func (e *OutdatedError) Error() string {
	which := "an older"
	if e.Newer {
		which = "a newer"
	}
	return fmt.Sprintf("%s was created by %s version; delete it (rm %s) and restart", e.Path, which, e.Path)
}

// Open checks the file at path (see the package doc), then opens it with
// store.Open (applying schema.sql) and stamps SchemaVersion. An out-of-date
// file is an *OutdatedError and is left untouched.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := Check(ctx, path); err != nil {
		return nil, err
	}
	db, err := store.Open(ctx, path, seatlane.Schema)
	if err != nil {
		return nil, err
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
		db.Close()
		return nil, fmt.Errorf("stamp schema version: %w", err)
	}
	return db, nil
}

// Check reports whether the existing file at path matches this version's
// schema: nil for a match, a missing file or a file with no tables; an
// *OutdatedError otherwise. It does not create or change the file.
func Check(ctx context.Context, path string) error {
	if path == ":memory:" {
		return nil
	}
	if st, err := os.Stat(path); errors.Is(err, os.ErrNotExist) || (err == nil && st.Size() == 0) {
		return nil
	} else if err != nil {
		return err
	}
	file, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	file.SetMaxOpenConns(1)

	have, err := tables(ctx, file)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if len(have) == 0 {
		return nil
	}
	var version int
	if err := file.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if version > SchemaVersion {
		return &OutdatedError{Path: path, Newer: true,
			Reason: fmt.Sprintf("schema version %d, this build knows %d", version, SchemaVersion)}
	}
	if version != 0 && version != SchemaVersion {
		return &OutdatedError{Path: path,
			Reason: fmt.Sprintf("schema version %d, want %d", version, SchemaVersion)}
	}

	want, err := expected(ctx)
	if err != nil {
		return err
	}
	if reason := diff(want, have); reason != "" {
		return &OutdatedError{Path: path, Reason: reason}
	}
	return nil
}

type column struct {
	Name, Type  string
	NotNull, PK bool
}

// expected is schema.sql's tables, read back from an in-memory database.
func expected(ctx context.Context) (map[string][]column, error) {
	mem, err := store.Open(ctx, ":memory:", seatlane.Schema)
	if err != nil {
		return nil, err
	}
	defer mem.Close()
	return tables(ctx, mem)
}

// tables lists the user tables of db and their columns in declaration order.
func tables(ctx context.Context, db *sql.DB) (map[string][]column, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make(map[string][]column, len(names))
	for _, n := range names {
		cols, err := db.QueryContext(ctx, `SELECT name, type, "notnull", pk FROM pragma_table_info(?)`, n)
		if err != nil {
			return nil, err
		}
		for cols.Next() {
			var c column
			var notNull, pk int
			if err := cols.Scan(&c.Name, &c.Type, &notNull, &pk); err != nil {
				cols.Close()
				return nil, err
			}
			c.Type, c.NotNull, c.PK = strings.ToUpper(c.Type), notNull != 0, pk != 0
			out[n] = append(out[n], c)
		}
		cols.Close()
		if err := cols.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// diff names the first difference between the wanted and the found tables
// ("" when every wanted table is there with the same columns): column
// differences in tables the file has first, then missing tables. Extra
// tables in the file are ignored.
func diff(want, have map[string][]column) string {
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		got, ok := have[n]
		if !ok {
			continue
		}
		byName := make(map[string]column, len(got))
		for _, c := range got {
			byName[c.Name] = c
		}
		for _, w := range want[n] {
			g, ok := byName[w.Name]
			switch {
			case !ok:
				return fmt.Sprintf("no %s.%s column", n, w.Name)
			case g.Type != w.Type:
				return fmt.Sprintf("%s.%s is %s, want %s", n, w.Name, g.Type, w.Type)
			case g.NotNull != w.NotNull || g.PK != w.PK:
				return fmt.Sprintf("%s.%s has other constraints", n, w.Name)
			}
		}
		if len(got) != len(want[n]) {
			return fmt.Sprintf("%s has %d columns, want %d", n, len(got), len(want[n]))
		}
	}
	for _, n := range names {
		if _, ok := have[n]; !ok {
			return fmt.Sprintf("no %s table", n)
		}
	}
	return ""
}

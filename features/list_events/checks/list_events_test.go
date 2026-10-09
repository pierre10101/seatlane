// Package checks holds the acceptance checks for List events, against a
// real SQLite database. Test names carry the F-IDs they cover.
package checks

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/page"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/list_events"
	"github.com/pierre10101/seatlane/features/list_events/db"
	"github.com/pierre10101/seatlane/internal/seed"
	"github.com/pierre10101/seatlane/internal/testkit"
)

func newAction(t *testing.T) (*list_events.Action, *sql.DB) {
	conn := testkit.Open(t, 0) // event 1 "Check Night"
	if err := seed.Load(context.Background(), conn, seed.Events); err != nil {
		t.Fatal(err)
	}
	testkit.Exec(t, conn, `UPDATE events SET on_sale = 0 WHERE id = 1`)
	return list_events.New(db.New(txn.DB(conn))), conn
}

func list(a *list_events.Action, after, limit int64) (list_events.Output, error) {
	in := list_events.Input{After: after, Limit: limit}
	return txn.Read(context.Background(), func(ctx context.Context) (list_events.Output, error) { return a.Handle(ctx, in) })
}

func TestListsEventsOnSaleNewestFirst(t *testing.T) {
	a, _ := newAction(t)
	out, err := list(a, page.StartCursor, 2)
	if err != nil || len(out.Events) != 2 || out.Events[0].EventID != 4 || out.NextAfter != 3 {
		t.Fatalf("out %+v err %v", out, err)
	}
	if e := out.Events[1]; e.Name != "Highveld Jazz Nights" || e.FromPrice.Cents != 28000 || e.ToPrice.Cents != 55000 || e.ToPrice.Currency != "ZAR" || e.FromPrice.Currency != "ZAR" {
		t.Fatalf("card %+v", e)
	}
	rest, err := list(a, out.NextAfter, 2)
	if err != nil || len(rest.Events) != 1 || rest.NextAfter != 0 {
		t.Fatalf("rest %+v err %v (event 1 is not on sale)", rest, err)
	}
}

func TestF11_PageLimitOutOfRange(t *testing.T) {
	a, _ := newAction(t)
	if _, err := list(a, page.StartCursor, 0); !errors.Is(err, list_events.F11) {
		t.Fatalf("want F11, got %v", err)
	}
}

func TestF12_BadCursor(t *testing.T) {
	a, _ := newAction(t)
	if _, err := list(a, -5, 10); !errors.Is(err, list_events.F12) {
		t.Fatalf("want F12, got %v", err)
	}
}

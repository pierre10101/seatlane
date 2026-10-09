// Package checks holds the acceptance checks for Release hold, against a
// real SQLite database. Test names carry the F-IDs they cover.
package checks

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/release_hold"
	"github.com/pierre10101/seatlane/features/release_hold/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

const t0 = testkit.T0

func newAction(t *testing.T) (*release_hold.Action, *sql.DB) {
	conn := testkit.Open(t, 3)
	testkit.Exec(t, conn, `UPDATE seats SET held_by = ?, held_at = ?, expires_at = ? WHERE id = 1`, testkit.Alice, t0, t0+600)
	testkit.Exec(t, conn, `UPDATE seats SET held_by = ?, held_at = ?, expires_at = ?, sold_to = ?, sold_at = ? WHERE id = 2`,
		testkit.Alice, t0, t0+600, testkit.Alice, t0+5)
	return release_hold.New(db.New(txn.DB(conn))), conn
}

func release(a *release_hold.Action, seat int64, session string, now int64) (release_hold.Output, error) {
	in := release_hold.Input{SeatID: seat, Session: session, Now: now}
	return txn.Run(context.Background(), func(ctx context.Context) (release_hold.Output, error) { return a.Handle(ctx, in) })
}

func TestReleasesOwnHold(t *testing.T) {
	a, conn := newAction(t)
	out, err := release(a, 1, testkit.Alice, t0+30)
	if err != nil || out != (release_hold.Output{SeatID: 1, ReleasedAt: t0 + 30, Now: t0 + 30}) {
		t.Fatalf("out %+v err %v", out, err)
	}
	if r := testkit.Seat(t, conn, 1); r != (testkit.Row{}) {
		t.Fatalf("stored %+v", r)
	}
}

func TestF5_ConfirmedSeatIsNotReleased(t *testing.T) {
	a, conn := newAction(t)
	before := testkit.Seat(t, conn, 2)
	if _, err := release(a, 2, testkit.Alice, t0+30); !errors.Is(err, release_hold.F5) {
		t.Fatalf("want F5, got %v", err)
	}
	if testkit.Seat(t, conn, 2) != before {
		t.Fatal("seat changed")
	}
}

func TestF6_SoldToSomeoneElse(t *testing.T) {
	a, _ := newAction(t)
	if _, err := release(a, 2, testkit.Bob, t0+30); !errors.Is(err, release_hold.F6) {
		t.Fatalf("want F6, got %v", err)
	}
}

func TestF7_NoSuchSeat(t *testing.T) {
	a, _ := newAction(t)
	if _, err := release(a, 99, testkit.Alice, t0); !errors.Is(err, release_hold.F7) {
		t.Fatalf("want F7, got %v", err)
	}
}

func TestF8_SessionRequired(t *testing.T) {
	a, _ := newAction(t)
	if _, err := release(a, 1, "", t0); !errors.Is(err, release_hold.F8) {
		t.Fatalf("want F8, got %v", err)
	}
}

// F9: Bob has no hold on Alice's seat; nobody holds seat 3; releasing twice.
func TestF9_NoHoldToRelease(t *testing.T) {
	a, conn := newAction(t)
	before := testkit.Seat(t, conn, 1)
	if _, err := release(a, 1, testkit.Bob, t0+30); !errors.Is(err, release_hold.F9) {
		t.Fatalf("someone else's hold: want F9, got %v", err)
	}
	if testkit.Seat(t, conn, 1) != before {
		t.Fatal("seat changed")
	}
	if _, err := release(a, 3, testkit.Alice, t0); !errors.Is(err, release_hold.F9) {
		t.Fatalf("free seat: want F9, got %v", err)
	}
	if _, err := release(a, 1, testkit.Alice, t0+30); err != nil {
		t.Fatal(err)
	}
	if _, err := release(a, 1, testkit.Alice, t0+31); !errors.Is(err, release_hold.F9) {
		t.Fatalf("released twice: want F9, got %v", err)
	}
}

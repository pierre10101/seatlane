// Package checks holds the acceptance checks for Confirm hold, against a
// real SQLite database. Holds are taken and released with the real
// hold_seat and release_hold actions. Test names carry the F-IDs they cover.
package checks

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/confirm_hold"
	"github.com/pierre10101/seatlane/features/confirm_hold/db"
	"github.com/pierre10101/seatlane/features/hold_seat"
	holddb "github.com/pierre10101/seatlane/features/hold_seat/db"
	"github.com/pierre10101/seatlane/features/release_hold"
	releasedb "github.com/pierre10101/seatlane/features/release_hold/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

const t0 = testkit.T0

type world struct {
	t       *testing.T
	conn    *sql.DB
	confirm *confirm_hold.Action
	hold    *hold_seat.Action
	release *release_hold.Action
}

func newWorld(t *testing.T) *world {
	conn := testkit.Open(t, 3)
	return &world{t, conn,
		confirm_hold.New(db.New(txn.DB(conn))),
		hold_seat.New(holddb.New(txn.DB(conn))),
		release_hold.New(releasedb.New(txn.DB(conn)))}
}

func run[I, O any](handle func(context.Context, I) (O, error), in I) (O, error) {
	return txn.Run(context.Background(), func(ctx context.Context) (O, error) { return handle(ctx, in) })
}

func (w *world) doConfirm(seat int64, session string, now int64) (confirm_hold.Output, error) {
	return run(w.confirm.Handle, confirm_hold.Input{SeatID: seat, Session: session, Now: now})
}

func (w *world) doHold(seat int64, session string, now int64) {
	w.t.Helper()
	if _, err := run(w.hold.Handle, hold_seat.Input{SeatID: seat, Session: session, Now: now}); err != nil {
		w.t.Fatalf("hold: %v", err)
	}
}

// expectUnchanged runs fn and fails if seat changed.
func (w *world) expectUnchanged(seat int64, fn func()) {
	w.t.Helper()
	before := testkit.Seat(w.t, w.conn, seat)
	fn()
	if after := testkit.Seat(w.t, w.conn, seat); after != before {
		w.t.Fatalf("seat %d changed: %+v -> %+v", seat, before, after)
	}
}

func TestConfirmsOwnLiveHold(t *testing.T) {
	w := newWorld(t)
	w.doHold(1, testkit.Alice, t0)
	out, err := w.doConfirm(1, testkit.Alice, t0+599)
	if err != nil || out != (confirm_hold.Output{SeatID: 1, SoldAt: t0 + 599, Now: t0 + 599}) {
		t.Fatalf("out %+v err %v", out, err)
	}
	if r := testkit.Seat(t, w.conn, 1); r.SoldTo != testkit.Alice || r.SoldAt != t0+599 {
		t.Fatalf("stored %+v", r)
	}
}

// F2: a hold taken 600 seconds or more before now has expired; confirming
// at exactly expires_at is too late. Nothing is sold.
func TestF2_HoldExpired(t *testing.T) {
	w := newWorld(t)
	w.doHold(1, testkit.Alice, t0)
	for _, later := range []int64{600, 601, 3600} {
		w.expectUnchanged(1, func() {
			if _, err := w.doConfirm(1, testkit.Alice, t0+later); !errors.Is(err, confirm_hold.F2) {
				t.Fatalf("%d s later: want F2, got %v", later, err)
			}
		})
	}
}

// F3: confirm after release (and confirm of a seat never held).
func TestF3_ConfirmAfterRelease(t *testing.T) {
	w := newWorld(t)
	w.doHold(1, testkit.Alice, t0)
	if _, err := run(w.release.Handle, release_hold.Input{SeatID: 1, Session: testkit.Alice, Now: t0 + 10}); err != nil {
		t.Fatal(err)
	}
	w.expectUnchanged(1, func() {
		if _, err := w.doConfirm(1, testkit.Alice, t0+20); !errors.Is(err, confirm_hold.F3) {
			t.Fatalf("after release: want F3, got %v", err)
		}
	})
	w.expectUnchanged(2, func() {
		if _, err := w.doConfirm(2, testkit.Alice, t0); !errors.Is(err, confirm_hold.F3) {
			t.Fatalf("never held: want F3, got %v", err)
		}
	})
}

// F4: Bob cannot confirm Alice's hold, live or expired.
func TestF4_ConfirmAnotherPersonsHold(t *testing.T) {
	w := newWorld(t)
	w.doHold(1, testkit.Alice, t0)
	for _, later := range []int64{1, 599, 600} {
		w.expectUnchanged(1, func() {
			if _, err := w.doConfirm(1, testkit.Bob, t0+later); !errors.Is(err, confirm_hold.F4) {
				t.Fatalf("%d s later: want F4, got %v", later, err)
			}
		})
	}
}

// F5: the second confirm of the same seat by the same session.
func TestF5_DoubleConfirm(t *testing.T) {
	w := newWorld(t)
	w.doHold(1, testkit.Alice, t0)
	if _, err := w.doConfirm(1, testkit.Alice, t0+5); err != nil {
		t.Fatal(err)
	}
	w.expectUnchanged(1, func() {
		if _, err := w.doConfirm(1, testkit.Alice, t0+6); !errors.Is(err, confirm_hold.F5) {
			t.Fatalf("want F5, got %v", err)
		}
	})
}

// F6: the seat was sold to someone else.
func TestF6_SoldToSomeoneElse(t *testing.T) {
	w := newWorld(t)
	w.doHold(1, testkit.Alice, t0)
	if _, err := w.doConfirm(1, testkit.Alice, t0+5); err != nil {
		t.Fatal(err)
	}
	w.expectUnchanged(1, func() {
		if _, err := w.doConfirm(1, testkit.Bob, t0+6); !errors.Is(err, confirm_hold.F6) {
			t.Fatalf("want F6, got %v", err)
		}
	})
}

func TestF7_NoSuchSeat(t *testing.T) {
	w := newWorld(t)
	if _, err := w.doConfirm(99, testkit.Alice, t0); !errors.Is(err, confirm_hold.F7) {
		t.Fatalf("want F7, got %v", err)
	}
}

func TestF8_SessionRequired(t *testing.T) {
	w := newWorld(t)
	w.expectUnchanged(1, func() {
		if _, err := w.doConfirm(1, "", t0); !errors.Is(err, confirm_hold.F8) {
			t.Fatalf("want F8, got %v", err)
		}
	})
}

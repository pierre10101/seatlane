// Package checks holds the acceptance checks for Confirm holds, against a
// real SQLite database. Holds are taken, released and confirmed with the real
// hold_seat, release_hold and confirm_hold actions. Test names carry the
// F-IDs they cover.
package checks

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane"
	"github.com/pierre10101/seatlane/features/confirm_hold"
	onedb "github.com/pierre10101/seatlane/features/confirm_hold/db"
	"github.com/pierre10101/seatlane/features/confirm_holds"
	"github.com/pierre10101/seatlane/features/confirm_holds/db"
	"github.com/pierre10101/seatlane/features/hold_seat"
	holddb "github.com/pierre10101/seatlane/features/hold_seat/db"
	"github.com/pierre10101/seatlane/features/release_hold"
	releasedb "github.com/pierre10101/seatlane/features/release_hold/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

const (
	t0    = testkit.T0
	seats = 8
)

type world struct {
	t       *testing.T
	conn    *sql.DB
	confirm *confirm_holds.Action
	one     *confirm_hold.Action
	hold    *hold_seat.Action
	release *release_hold.Action
}

func newWorld(t *testing.T) *world {
	return worldOn(t, testkit.Open(t, seats))
}

func worldOn(t *testing.T, conn *sql.DB) *world {
	return &world{t, conn,
		confirm_holds.New(db.New(txn.DB(conn))),
		confirm_hold.New(onedb.New(txn.DB(conn))),
		hold_seat.New(holddb.New(txn.DB(conn))),
		release_hold.New(releasedb.New(txn.DB(conn)))}
}

func run[I, O any](handle func(context.Context, I) (O, error), in I) (O, error) {
	return txn.Run(context.Background(), func(ctx context.Context) (O, error) { return handle(ctx, in) })
}

func (w *world) doConfirm(session string, now int64, ids ...int64) (confirm_holds.Output, error) {
	return run(w.confirm.Handle, confirm_holds.Input{SeatIDs: ids, Session: session, Now: now})
}

func (w *world) doHold(session string, now int64, ids ...int64) {
	w.t.Helper()
	for _, id := range ids {
		if _, err := run(w.hold.Handle, hold_seat.Input{SeatID: id, Session: session, Now: now}); err != nil {
			w.t.Fatalf("hold %d: %v", id, err)
		}
	}
}

func (w *world) snapshot() []testkit.Row {
	rows := make([]testkit.Row, seats+1)
	for id := int64(1); id <= seats; id++ {
		rows[id] = testkit.Seat(w.t, w.conn, id)
	}
	return rows
}

// expectNothingWritten runs fn and fails if any seat changed.
func (w *world) expectNothingWritten(fn func()) {
	w.t.Helper()
	before := w.snapshot()
	fn()
	after := w.snapshot()
	for id := 1; id <= seats; id++ {
		if after[id] != before[id] {
			w.t.Fatalf("seat %d changed: %+v -> %+v (something was written)", id, before[id], after[id])
		}
	}
}

func (w *world) expectSold(session string, at int64, ids ...int64) {
	w.t.Helper()
	for _, id := range ids {
		if r := testkit.Seat(w.t, w.conn, id); r.SoldTo != session || r.SoldAt != at {
			w.t.Fatalf("seat %d: %+v, want sold to %s at %d", id, r, session, at)
		}
	}
}

// All confirmed: every listed seat is sold to the holder at the same second.
func TestConfirmsAllListedSeats(t *testing.T) {
	w := newWorld(t)
	w.doHold(testkit.Alice, t0, 1, 2, 3)
	out, err := w.doConfirm(testkit.Alice, t0+599, 3, 1, 2)
	if err != nil || out != (confirm_holds.Output{Confirmed: 3, SoldAt: t0 + 599, Now: t0 + 599}) {
		t.Fatalf("out %+v err %v", out, err)
	}
	w.expectSold(testkit.Alice, t0+599, 1, 2, 3)
}

// A subset: only the listed seats are sold; the other holds are untouched.
func TestConfirmsOnlyTheListedSubset(t *testing.T) {
	w := newWorld(t)
	w.doHold(testkit.Alice, t0, 1, 2, 3, 4)
	before := w.snapshot()
	out, err := w.doConfirm(testkit.Alice, t0+10, 1, 3)
	if err != nil || out.Confirmed != 2 {
		t.Fatalf("out %+v err %v", out, err)
	}
	w.expectSold(testkit.Alice, t0+10, 1, 3)
	for _, id := range []int64{2, 4} {
		if r := testkit.Seat(t, w.conn, id); r != before[id] {
			t.Fatalf("seat %d not in the list changed: %+v -> %+v", id, before[id], r)
		}
	}
}

// F2: one expired hold among live ones: nothing is sold, the live holds are
// rolled back and stay held. The boundary is exact: expires_at = now has
// expired, one second before it has not.
func TestF2_OneHoldExpiredNothingWritten(t *testing.T) {
	w := newWorld(t)
	w.doHold(testkit.Alice, t0, 1)
	w.doHold(testkit.Alice, t0+300, 2, 3)
	for _, now := range []int64{t0 + 600, t0 + 601, t0 + 3600} {
		w.expectNothingWritten(func() {
			if _, err := w.doConfirm(testkit.Alice, now, 2, 1, 3); !errors.Is(err, confirm_holds.F2) {
				t.Fatalf("at %d: want F2, got %v", now-t0, err)
			}
		})
	}
	if r := testkit.Seat(t, w.conn, 2); r.HeldBy != testkit.Alice || r.SoldTo != "" {
		t.Fatalf("live hold lost: %+v", r)
	}
	if _, err := w.doConfirm(testkit.Alice, t0+599, 1, 2, 3); err != nil {
		t.Fatalf("one second before it expires: %v", err)
	}
	w.expectSold(testkit.Alice, t0+599, 1, 2, 3)
}

// F2 wins over F13 when the list also has a seat that is not mine.
func TestF2_ExpiredAndForeignIsF2(t *testing.T) {
	w := newWorld(t)
	w.doHold(testkit.Alice, t0, 1, 2)
	w.doHold(testkit.Bob, t0+500, 3)
	w.expectNothingWritten(func() {
		if _, err := w.doConfirm(testkit.Alice, t0+600, 1, 2, 3); !errors.Is(err, confirm_holds.F2) {
			t.Fatalf("want F2, got %v", err)
		}
	})
}

// F13: a listed seat this session does not hold: someone else's (live or
// expired), released, never held, already sold to me, sold to someone else,
// or no such seat. My other listed holds are rolled back: nothing is written.
func TestF13_SeatNotMineRollsBackEverything(t *testing.T) {
	for name, tc := range map[string]struct {
		setup func(w *world)
		ids   []int64
		now   int64
	}{
		"held by someone else":        {func(w *world) { w.doHold(testkit.Bob, t0, 5) }, []int64{1, 2, 5}, t0 + 10},
		"someone else's expired hold": {func(w *world) { w.doHold(testkit.Bob, t0-600, 5) }, []int64{1, 5}, t0 + 10},
		"never held":                  {func(w *world) {}, []int64{1, 2, 6}, t0 + 10},
		"released": {func(w *world) {
			w.doHold(testkit.Alice, t0, 5)
			if _, err := run(w.release.Handle, release_hold.Input{SeatID: 5, Session: testkit.Alice, Now: t0 + 1}); err != nil {
				t.Fatal(err)
			}
		}, []int64{5, 1}, t0 + 10},
		"already sold to me": {func(w *world) {
			w.doHold(testkit.Alice, t0, 5)
			if _, err := run(w.one.Handle, confirm_hold.Input{SeatID: 5, Session: testkit.Alice, Now: t0 + 1}); err != nil {
				t.Fatal(err)
			}
		}, []int64{1, 2, 5}, t0 + 10},
		"sold to someone else": {func(w *world) {
			w.doHold(testkit.Bob, t0, 5)
			if _, err := run(w.one.Handle, confirm_hold.Input{SeatID: 5, Session: testkit.Bob, Now: t0 + 1}); err != nil {
				t.Fatal(err)
			}
		}, []int64{1, 5}, t0 + 10},
		"no such seat": {func(w *world) {}, []int64{1, 2, 99}, t0 + 10},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.doHold(testkit.Alice, t0, 1, 2)
			tc.setup(w)
			w.expectNothingWritten(func() {
				if _, err := w.doConfirm(testkit.Alice, tc.now, tc.ids...); !errors.Is(err, confirm_holds.F13) {
					t.Fatalf("want F13, got %v", err)
				}
			})
		})
	}
}

// Handle called directly with a repeated id (httpx.Bind answers 400, see
// http_test.go): the count cannot match, so F13 and nothing written.
func TestF13_RepeatedIDCalledDirectly(t *testing.T) {
	w := newWorld(t)
	w.doHold(testkit.Alice, t0, 1)
	w.expectNothingWritten(func() {
		if _, err := w.doConfirm(testkit.Alice, t0+1, 1, 1); !errors.Is(err, confirm_holds.F13) {
			t.Fatalf("want F13, got %v", err)
		}
	})
}

func TestF8_SessionRequired(t *testing.T) {
	w := newWorld(t)
	w.doHold(testkit.Alice, t0, 1)
	w.expectNothingWritten(func() {
		if _, err := w.doConfirm("", t0+1, 1); !errors.Is(err, confirm_holds.F8) {
			t.Fatalf("want F8, got %v", err)
		}
	})
}

// Concurrency: separate connections to one database file. Eight group
// confirms of the same seats and one single-seat confirm_hold of one of them
// race with Bob's group confirm of his own seats. The claim checks and writes
// in one statement inside a BEGIN IMMEDIATE transaction, so Alice's seats are
// sold exactly once, either all by one group call or (if the single confirm
// won) none by any group call; Bob's disjoint group is not disturbed.
func TestF13_ConcurrentConfirmsSellOnce(t *testing.T) {
	w := newWorld(t)
	w.doHold(testkit.Alice, t0, 1, 2, 3)
	w.doHold(testkit.Bob, t0, 4, 5)
	var path string
	if err := w.conn.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil || path == "" {
		t.Fatalf("db path %q: %v", path, err)
	}
	other := func() *world {
		conn, err := store.Open(context.Background(), path, seatlane.Schema)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		return worldOn(t, conn)
	}
	const n = 8
	errs := make([]error, n+2)
	worlds := make([]*world, n+2)
	for i := range worlds {
		worlds[i] = other()
	}
	var wg sync.WaitGroup
	for i := 0; i < n+2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i {
			case n:
				_, errs[i] = worlds[i].doConfirm(testkit.Bob, t0+5, 4, 5)
			case n + 1:
				_, errs[i] = run(worlds[i].one.Handle, confirm_hold.Input{SeatID: 2, Session: testkit.Alice, Now: t0 + 5})
			default:
				_, errs[i] = worlds[i].doConfirm(testkit.Alice, t0+5, 1, 2, 3)
			}
		}(i)
	}
	wg.Wait()
	groups := 0
	for i, err := range errs[:n] {
		switch {
		case err == nil:
			groups++
		case !errors.Is(err, confirm_holds.F13):
			t.Fatalf("group call %d: want success or F13, got %v", i, err)
		}
	}
	if errs[n] != nil {
		t.Fatalf("Bob's group: %v", errs[n])
	}
	w.expectSold(testkit.Bob, t0+5, 4, 5)
	single := errs[n+1]
	switch {
	case groups == 1 && errors.Is(single, confirm_hold.F5):
		w.expectSold(testkit.Alice, t0+5, 1, 2, 3)
	case groups == 0 && single == nil:
		w.expectSold(testkit.Alice, t0+5, 2)
		for _, id := range []int64{1, 3} {
			if r := testkit.Seat(t, w.conn, id); r.SoldTo != "" || r.HeldBy != testkit.Alice {
				t.Fatalf("seat %d: %+v, want still held and unsold", id, r)
			}
		}
	default:
		t.Fatalf("group confirms succeeded %d times, single confirm: %v", groups, single)
	}
}

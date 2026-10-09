// Package checks holds the acceptance checks for Hold seat, against a real
// SQLite database. Test names carry the F-IDs they cover: TestF<n>_...
package checks

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/hold_seat"
	"github.com/pierre10101/seatlane/features/hold_seat/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

const t0 = testkit.T0

func newAction(t *testing.T) (*hold_seat.Action, *sql.DB) {
	t.Helper()
	conn := testkit.Open(t, 3)
	return hold_seat.New(db.New(txn.DB(conn))), conn
}

// hold calls the action the way httpx.Bind does: in one transaction.
func hold(a *hold_seat.Action, seat int64, user int64, now int64) (hold_seat.Output, error) {
	in := hold_seat.Input{SeatID: seat, User: user, Now: now}
	return txn.Run(context.Background(), func(ctx context.Context) (hold_seat.Output, error) { return a.Handle(ctx, in) })
}

func TestHoldsAFreeSeatForTenMinutes(t *testing.T) {
	a, conn := newAction(t)
	out, err := hold(a, 1, testkit.Alice, t0)
	if err != nil || out != (hold_seat.Output{SeatID: 1, HeldAt: t0, ExpiresAt: t0 + 600, Now: t0}) {
		t.Fatalf("out %+v err %v", out, err)
	}
	if r := testkit.Seat(t, conn, 1); r != (testkit.Row{HeldBy: testkit.Alice, HeldAt: t0, ExpiresAt: t0 + 600}) {
		t.Fatalf("stored %+v", r)
	}
}

// F1: a hold blocks the seat until its expires_at (= held_at + 600). At
// 599 seconds it still blocks, for others and for the holder; at exactly
// 600 seconds it has expired and the seat can be held again.
func TestF1_SeatAlreadyHeld(t *testing.T) {
	a, conn := newAction(t)
	if _, err := hold(a, 1, testkit.Alice, t0); err != nil {
		t.Fatal(err)
	}
	for _, try := range []struct {
		user  int64
		later int64
	}{
		{testkit.Bob, 0}, {testkit.Bob, 1}, {testkit.Bob, 599}, {testkit.Alice, 30},
	} {
		if _, err := hold(a, 1, try.user, t0+try.later); !errors.Is(err, hold_seat.F1) {
			t.Fatalf("user %d, %d s later: want F1, got %v", try.user, try.later, err)
		}
		if r := testkit.Seat(t, conn, 1); r.HeldBy != testkit.Alice || r.HeldAt != t0 || r.ExpiresAt != t0+600 {
			t.Fatalf("%d s later: hold changed to %+v", try.later, r)
		}
	}
	out, err := hold(a, 1, testkit.Bob, t0+600)
	if err != nil || out.ExpiresAt != t0+1200 {
		t.Fatalf("exactly 600 s later the hold has expired: out %+v err %v", out, err)
	}
	if r := testkit.Seat(t, conn, 1); r.HeldBy != testkit.Bob {
		t.Fatalf("stored %+v", r)
	}
}

// F1 under contention: many customers at the same second, exactly one wins.
func TestF1_ConcurrentHoldsExactlyOneWins(t *testing.T) {
	a, conn := newAction(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, f1 := 0, 0
	for s := int64(1); s <= 24; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := hold(a, 2, 100+s, t0)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, hold_seat.F1):
				f1++
			default:
				t.Errorf("unexpected error %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 || f1 != 23 {
		t.Fatalf("wins %d, F1 %d", wins, f1)
	}
	if r := testkit.Seat(t, conn, 2); r.HeldBy < 101 || r.HeldBy > 124 {
		t.Fatalf("stored %+v", r)
	}
}

func TestF6_SeatAlreadySold(t *testing.T) {
	a, conn := newAction(t)
	testkit.Exec(t, conn, `UPDATE seats SET held_by = ?, held_at = ?, expires_at = ?, sold_to = ?, sold_at = ? WHERE id = 1`,
		testkit.Bob, t0-900, t0-300, testkit.Bob, t0-400)
	before := testkit.Seat(t, conn, 1)
	for _, s := range []int64{testkit.Alice, testkit.Bob} {
		if _, err := hold(a, 1, s, t0); !errors.Is(err, hold_seat.F6) {
			t.Fatalf("user %d: want F6, got %v", s, err)
		}
	}
	if r := testkit.Seat(t, conn, 1); r != before {
		t.Fatalf("sold seat changed: %+v", r)
	}
}

func TestF7_NoSuchSeat(t *testing.T) {
	a, _ := newAction(t)
	if _, err := hold(a, 99, testkit.Alice, t0); !errors.Is(err, hold_seat.F7) {
		t.Fatalf("want F7, got %v", err)
	}
}

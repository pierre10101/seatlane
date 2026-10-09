// Package checks holds the acceptance checks for Show seat map, against a
// real SQLite database. Test names carry the F-IDs they cover.
package checks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/page"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/show_seat_map"
	"github.com/pierre10101/seatlane/features/show_seat_map/db"
	"github.com/pierre10101/seatlane/internal/domain"
	"github.com/pierre10101/seatlane/internal/session"
	"github.com/pierre10101/seatlane/internal/testkit"
)

const t0 = testkit.T0

func newAction(t *testing.T, n int) (*show_seat_map.Action, *sql.DB) {
	conn := testkit.Open(t, n)
	return show_seat_map.New(db.New(txn.DB(conn))), conn
}

func show(a *show_seat_map.Action, in show_seat_map.Input) (show_seat_map.Output, error) {
	return txn.Read(context.Background(), func(ctx context.Context) (show_seat_map.Output, error) { return a.Handle(ctx, in) })
}

func states(s domain.SeatView) [5]bool {
	return [5]bool{s.Available, s.HeldByMe, s.HeldByOther, s.SoldToMe, s.SoldToOther}
}

// The server decides every seat state; exactly one flag is true. A hold is
// active while expires_at is later than now.
func TestSeatStatesForTheViewer(t *testing.T) {
	a, conn := newAction(t, 7)
	set := func(id, heldBy, expires, soldTo int64) {
		testkit.Exec(t, conn, `UPDATE seats SET held_by = ?, held_at = ?, expires_at = ?, sold_to = ? WHERE id = ?`, heldBy, expires-600, expires, soldTo, id)
	}
	set(2, testkit.Alice, t0+1, 0)              // mine, 1 s left
	set(3, testkit.Bob, t0+300, 0)              // Bob's
	set(4, testkit.Bob, t0, 0)                  // Bob's, expired exactly now
	set(5, testkit.Alice, t0-1, 0)              // mine, expired
	set(6, testkit.Alice, t0-10, testkit.Alice) // sold to me
	set(7, testkit.Bob, t0-10, testkit.Bob)     // sold to Bob
	out, err := show(a, show_seat_map.Input{EventID: 1, After: page.StartCursor, Limit: 20, Session: testkit.Alice, Now: t0})
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64][5]bool{
		1: {true, false, false, false, false}, 2: {false, true, false, false, false},
		3: {false, false, true, false, false}, 4: {true, false, false, false, false},
		5: {true, false, false, false, false}, 6: {false, false, false, true, false},
		7: {false, false, false, false, true},
	}
	if len(out.Seats) != 7 || out.NextAfter != 0 || out.Now != t0 || out.Event.Name != "Check Night" {
		t.Fatalf("out %+v", out)
	}
	for _, s := range out.Seats {
		if states(s) != want[s.SeatID] {
			t.Errorf("seat %d: states %v want %v", s.SeatID, states(s), want[s.SeatID])
		}
	}
	if out.Seats[0].SeatID != 7 || out.Seats[5].ExpiresAt != t0+1 || out.Seats[0].Price != (domain.Money{Cents: 45000, Currency: "ZAR"}) {
		t.Fatalf("order or fields: %+v", out.Seats)
	}
}

func TestPagesThroughSeats(t *testing.T) {
	a, _ := newAction(t, 5)
	first, err := show(a, show_seat_map.Input{EventID: 1, After: page.StartCursor, Limit: 3, Session: 1, Now: t0})
	if err != nil || len(first.Seats) != 3 || first.NextAfter != 3 {
		t.Fatalf("first %+v %v", first, err)
	}
	second, err := show(a, show_seat_map.Input{EventID: 1, After: first.NextAfter, Limit: 3, Session: 1, Now: t0})
	if err != nil || len(second.Seats) != 2 || second.NextAfter != 0 || second.Seats[0].SeatID != 2 {
		t.Fatalf("second %+v %v", second, err)
	}
}

func TestF8_SessionRequired(t *testing.T) {
	a, _ := newAction(t, 1)
	if _, err := show(a, show_seat_map.Input{EventID: 1, After: page.StartCursor, Limit: 20, Session: 0, Now: t0}); !errors.Is(err, show_seat_map.F8) {
		t.Fatalf("want F8, got %v", err)
	}
}

func TestF10_NoSuchEvent(t *testing.T) {
	a, _ := newAction(t, 1)
	if _, err := show(a, show_seat_map.Input{EventID: 9, After: page.StartCursor, Limit: 20, Session: 1, Now: t0}); !errors.Is(err, show_seat_map.F10) {
		t.Fatalf("want F10, got %v", err)
	}
}

func TestF11_PageLimitOutOfRange(t *testing.T) {
	a, _ := newAction(t, 1)
	for _, l := range []int64{0, 101} {
		if _, err := show(a, show_seat_map.Input{EventID: 1, After: page.StartCursor, Limit: l, Session: 1, Now: t0}); !errors.Is(err, show_seat_map.F11) {
			t.Fatalf("limit %d: want F11, got %v", l, err)
		}
	}
}

func TestF12_BadCursor(t *testing.T) {
	a, _ := newAction(t, 1)
	if _, err := show(a, show_seat_map.Input{EventID: 1, After: 0, Limit: 20, Session: 1, Now: t0}); !errors.Is(err, show_seat_map.F12) {
		t.Fatalf("want F12, got %v", err)
	}
}

// Over HTTP: the session comes from the cookie, now from the server clock,
// and a caller-sent session is refused.
func TestHTTPSeatMapUsesCookieSession(t *testing.T) {
	_, conn := newAction(t, 2)
	testkit.Exec(t, conn, `UPDATE seats SET held_by = 1001, held_at = ?, expires_at = ? WHERE id = 1`, t0, t0+600)
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0+10, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(show_seat_map.Route, httpx.Bind(show_seat_map.New(db.New(txn.DB(conn))).Handle))
	h := session.Wrap(mux)
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: session.Cookie, Value: "1001"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := get("/api/events/1/seats?session=2002"); rec.Code != http.StatusBadRequest {
		t.Fatalf("caller-sent session: %d", rec.Code)
	}
	rec := get("/api/events/1/seats")
	var out show_seat_map.Output
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Seats) != 2 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if !out.Seats[1].HeldByMe || out.Seats[1].ExpiresAt-out.Now != 590 {
		t.Fatalf("seat 1 %+v now %d", out.Seats[1], out.Now)
	}
}

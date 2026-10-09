// Package checks holds the acceptance checks for List my holds, against a
// real SQLite database. Test names carry the F-IDs they cover.
package checks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/page"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/list_my_holds"
	"github.com/pierre10101/seatlane/features/list_my_holds/db"
	"github.com/pierre10101/seatlane/internal/domain"
	"github.com/pierre10101/seatlane/internal/testkit"
)

const t0 = testkit.T0

func newAction(t *testing.T, n int) (*list_my_holds.Action, *sql.DB) {
	conn := testkit.Open(t, n)
	return list_my_holds.New(db.New(txn.DB(conn))), conn
}

func list(a *list_my_holds.Action, in list_my_holds.Input) (list_my_holds.Output, error) {
	return txn.Read(context.Background(), func(ctx context.Context) (list_my_holds.Output, error) { return a.Handle(ctx, in) })
}

// hold sets seat id's hold (held_at = expires - 600) and sale.
func hold(t *testing.T, conn *sql.DB, id int64, heldBy string, expires int64, soldTo string) {
	testkit.Exec(t, conn, `UPDATE seats SET held_by = ?, held_at = ?, expires_at = ?, sold_to = ? WHERE id = ?`, heldBy, expires-600, expires, soldTo, id)
}

// Only this session's unsold holds are listed, never anyone else's; a hold
// is active while expires_at is later than now (at expires_at it is not).
func TestListsOnlyMyHolds(t *testing.T) {
	a, conn := newAction(t, 6)
	hold(t, conn, 2, testkit.Alice, t0+1, "")             // mine, 1 s left
	hold(t, conn, 3, testkit.Bob, t0+300, "")             // Bob's
	hold(t, conn, 4, testkit.Alice, t0, "")               // mine, expired exactly now
	hold(t, conn, 5, testkit.Alice, t0-10, testkit.Alice) // sold to me: not a hold
	out, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: 20, Session: testkit.Alice, Now: t0})
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.MyHold{
		{SeatID: 4, HeldAt: t0 - 600, ExpiresAt: t0, Active: false},
		{SeatID: 2, HeldAt: t0 - 599, ExpiresAt: t0 + 1, Active: true},
	}
	if len(out.Holds) != len(want) || out.NextAfter != 0 || out.Now != t0 {
		t.Fatalf("out %+v", out)
	}
	for i := range want {
		if out.Holds[i] != want[i] {
			t.Fatalf("hold %d: %+v want %+v", i, out.Holds[i], want[i])
		}
	}
	bob, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: 20, Session: testkit.Bob, Now: t0})
	if err != nil || len(bob.Holds) != 1 || bob.Holds[0].SeatID != 3 {
		t.Fatalf("bob %+v %v", bob, err)
	}
	none, err := list(a, list_my_holds.Input{EventID: 9, After: page.StartCursor, Limit: 20, Session: testkit.Alice, Now: t0})
	if err != nil || none.Holds == nil || len(none.Holds) != 0 {
		t.Fatalf("no such event: %+v %v", none, err)
	}
}

func TestPagesThroughMyHolds(t *testing.T) {
	a, conn := newAction(t, 5)
	for id := int64(1); id <= 5; id++ {
		hold(t, conn, id, testkit.Alice, t0+600, "")
	}
	first, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: 3, Session: testkit.Alice, Now: t0})
	if err != nil || len(first.Holds) != 3 || first.NextAfter != 3 {
		t.Fatalf("first %+v %v", first, err)
	}
	second, err := list(a, list_my_holds.Input{EventID: 1, After: first.NextAfter, Limit: 3, Session: testkit.Alice, Now: t0})
	if err != nil || len(second.Holds) != 2 || second.NextAfter != 0 || second.Holds[0].SeatID != 2 {
		t.Fatalf("second %+v %v", second, err)
	}
}

func TestF8_SessionRequired(t *testing.T) {
	a, _ := newAction(t, 1)
	if _, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: 20, Session: "", Now: t0}); !errors.Is(err, list_my_holds.F8) {
		t.Fatalf("want F8, got %v", err)
	}
}

func TestF11_PageLimitOutOfRange(t *testing.T) {
	a, _ := newAction(t, 1)
	for _, l := range []int64{0, 101} {
		if _, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: l, Session: testkit.Alice, Now: t0}); !errors.Is(err, list_my_holds.F11) {
			t.Fatalf("limit %d: want F11, got %v", l, err)
		}
	}
}

func TestF12_BadCursor(t *testing.T) {
	a, _ := newAction(t, 1)
	if _, err := list(a, list_my_holds.Input{EventID: 1, After: 0, Limit: 20, Session: testkit.Alice, Now: t0}); !errors.Is(err, list_my_holds.F12) {
		t.Fatalf("want F12, got %v", err)
	}
}

// Over HTTP: the session comes from the cookie only; another visitor's
// cookie never shows my hold times.
func TestF8_HTTPMyHoldsUseCookieSession(t *testing.T) {
	_, conn := newAction(t, 2)
	hold(t, conn, 1, testkit.Alice, t0+600, "")
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0+10, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(list_my_holds.Route, httpx.Bind(list_my_holds.New(db.New(txn.DB(conn))).Handle))
	get := func(cookie, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: httpx.SessionCookie, Value: cookie})
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	if rec := get(testkit.Bob, "/api/events/1/holds?session="+testkit.Alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("caller-sent session: %d %s", rec.Code, rec.Body)
	}
	if rec := get("", "/api/events/1/holds"); rec.Code != list_my_holds.F8.Status || !strings.Contains(rec.Body.String(), `"`+list_my_holds.F8.ID+`"`) {
		t.Fatalf("no cookie: %d %s", rec.Code, rec.Body)
	}
	var out list_my_holds.Output
	if rec := get(testkit.Bob, "/api/events/1/holds"); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Holds) != 0 {
		t.Fatalf("bob: %d %s", rec.Code, rec.Body)
	}
	rec := get(testkit.Alice, "/api/events/1/holds")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Holds) != 1 ||
		out.Holds[0].ExpiresAt-out.Now != 590 || !out.Holds[0].Active {
		t.Fatalf("alice: %d %s", rec.Code, rec.Body)
	}
}

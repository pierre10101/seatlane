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
	"strconv"
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
func hold(t *testing.T, conn *sql.DB, id int64, heldBy int64, expires int64, soldTo int64) {
	testkit.Exec(t, conn, `UPDATE seats SET held_by = ?, held_at = ?, expires_at = ?, sold_to = ? WHERE id = ?`, heldBy, expires-600, expires, soldTo, id)
}

// Only this customer's unsold holds are listed, never anyone else's; a hold
// is active while expires_at is later than now (at expires_at it is not).
func TestListsOnlyMyHolds(t *testing.T) {
	a, conn := newAction(t, 6)
	hold(t, conn, 2, testkit.Alice, t0+1, 0)              // mine, 1 s left
	hold(t, conn, 3, testkit.Bob, t0+300, 0)              // Bob's
	hold(t, conn, 4, testkit.Alice, t0, 0)                // mine, expired exactly now
	hold(t, conn, 5, testkit.Alice, t0-10, testkit.Alice) // sold to me: not a hold
	out, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: 20, User: testkit.Alice, Now: t0})
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
	bob, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: 20, User: testkit.Bob, Now: t0})
	if err != nil || len(bob.Holds) != 1 || bob.Holds[0].SeatID != 3 {
		t.Fatalf("bob %+v %v", bob, err)
	}
	none, err := list(a, list_my_holds.Input{EventID: 9, After: page.StartCursor, Limit: 20, User: testkit.Alice, Now: t0})
	if err != nil || none.Holds == nil || len(none.Holds) != 0 {
		t.Fatalf("no such event: %+v %v", none, err)
	}
}

func TestPagesThroughMyHolds(t *testing.T) {
	a, conn := newAction(t, 5)
	for id := int64(1); id <= 5; id++ {
		hold(t, conn, id, testkit.Alice, t0+600, 0)
	}
	first, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: 3, User: testkit.Alice, Now: t0})
	if err != nil || len(first.Holds) != 3 || first.NextAfter != 3 {
		t.Fatalf("first %+v %v", first, err)
	}
	second, err := list(a, list_my_holds.Input{EventID: 1, After: first.NextAfter, Limit: 3, User: testkit.Alice, Now: t0})
	if err != nil || len(second.Holds) != 2 || second.NextAfter != 0 || second.Holds[0].SeatID != 2 {
		t.Fatalf("second %+v %v", second, err)
	}
}

func TestF11_PageLimitOutOfRange(t *testing.T) {
	a, _ := newAction(t, 1)
	for _, l := range []int64{0, 101} {
		if _, err := list(a, list_my_holds.Input{EventID: 1, After: page.StartCursor, Limit: l, User: testkit.Alice, Now: t0}); !errors.Is(err, list_my_holds.F11) {
			t.Fatalf("limit %d: want F11, got %v", l, err)
		}
	}
}

func TestF12_BadCursor(t *testing.T) {
	a, _ := newAction(t, 1)
	if _, err := list(a, list_my_holds.Input{EventID: 1, After: 0, Limit: 20, User: testkit.Alice, Now: t0}); !errors.Is(err, list_my_holds.F12) {
		t.Fatalf("want F12, got %v", err)
	}
}

// Over HTTP: only a signed-in customer may list holds (401 when not signed
// in, 403 for an organizer or admin); the customer is the signed-in user
// only, never a query value, and another customer never sees my hold times.
// F11 still answers over HTTP with its id.
func TestF11_HTTPMyHoldsAreTheSignedInCustomers(t *testing.T) {
	_, conn := newAction(t, 2)
	hold(t, conn, 1, testkit.Alice, t0+600, 0)
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0+10, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(list_my_holds.Route, httpx.Bind(list_my_holds.Roles, list_my_holds.New(db.New(txn.DB(conn))).Handle))
	h := testkit.Serve(mux)
	alice, bob := strconv.FormatInt(testkit.Alice, 10), strconv.FormatInt(testkit.Bob, 10)
	get := func(user, role, path string) *httptest.ResponseRecorder {
		return testkit.Do(h, http.MethodGet, path, "", user, role)
	}
	if rec := get(bob, "customer", "/api/events/1/holds?user="+alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("caller-sent user: %d %s", rec.Code, rec.Body)
	}
	for _, c := range []struct {
		user, role string
		code       int
		id         string
	}{
		{"", "", http.StatusUnauthorized, "unauthorized"},
		{strconv.FormatInt(testkit.Olga, 10), "organizer", http.StatusForbidden, "forbidden"},
		{strconv.FormatInt(testkit.Ada, 10), "admin", http.StatusForbidden, "forbidden"},
	} {
		if rec := get(c.user, c.role, "/api/events/1/holds"); rec.Code != c.code || testkit.ErrorID(rec) != c.id {
			t.Fatalf("%s %s: %d %s", c.user, c.role, rec.Code, rec.Body)
		}
	}
	if rec := get(alice, "customer", "/api/events/1/holds?limit=0"); rec.Code != list_my_holds.F11.Status || testkit.ErrorID(rec) != list_my_holds.F11.ID {
		t.Fatalf("limit 0: %d %s", rec.Code, rec.Body)
	}
	var out list_my_holds.Output
	if rec := get(bob, "customer", "/api/events/1/holds"); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Holds) != 0 {
		t.Fatalf("bob: %d %s", rec.Code, rec.Body)
	}
	rec := get(alice, "customer", "/api/events/1/holds")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Holds) != 1 ||
		out.Holds[0].ExpiresAt-out.Now != 590 || !out.Holds[0].Active {
		t.Fatalf("alice: %d %s", rec.Code, rec.Body)
	}
}

package checks

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/release_hold"
	"github.com/pierre10101/seatlane/features/release_hold/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

// Over HTTP only a signed-in customer may release: 401 when not signed in,
// 403 for an organizer or admin, nothing written. Another customer gets F9;
// the holder releases the seat.
func TestF9_HTTPReleaseIsForSignedInCustomers(t *testing.T) {
	_, conn := newAction(t)
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0+10, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(release_hold.Route, httpx.Bind(release_hold.Roles, release_hold.New(db.New(txn.DB(conn))).Handle))
	h := testkit.Serve(mux)
	before := testkit.Seat(t, conn, 1)
	for _, c := range []struct {
		user, role string
		code       int
		id         string
	}{
		{"", "", http.StatusUnauthorized, "unauthorized"},
		{strconv.FormatInt(testkit.Olga, 10), "organizer", http.StatusForbidden, "forbidden"},
		{strconv.FormatInt(testkit.Ada, 10), "admin", http.StatusForbidden, "forbidden"},
		{strconv.FormatInt(testkit.Bob, 10), "customer", release_hold.F9.Status, release_hold.F9.ID},
	} {
		if rec := testkit.Do(h, http.MethodPost, "/api/holds/release", `{"seat_id": 1}`, c.user, c.role); rec.Code != c.code || testkit.ErrorID(rec) != c.id {
			t.Fatalf("%s %s: %d %s, want %d %s", c.user, c.role, rec.Code, rec.Body, c.code, c.id)
		}
		if testkit.Seat(t, conn, 1) != before {
			t.Fatalf("%s %s changed the seat", c.user, c.role)
		}
	}
	if rec := testkit.Do(h, http.MethodPost, "/api/holds/release", `{"seat_id": 1}`, strconv.FormatInt(testkit.Alice, 10), "customer"); rec.Code != http.StatusCreated {
		t.Fatalf("holder: %d %s", rec.Code, rec.Body)
	}
	if r := testkit.Seat(t, conn, 1); r != (testkit.Row{}) {
		t.Fatalf("stored %+v", r)
	}
}

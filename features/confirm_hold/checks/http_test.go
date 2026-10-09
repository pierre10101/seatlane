package checks

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/confirm_hold"
	"github.com/pierre10101/seatlane/features/confirm_hold/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

// Over HTTP only a signed-in customer may confirm: 401 when not signed in,
// 403 "forbidden" for an organizer or admin, both before the action runs and
// without writing. A customer confirming someone else's hold gets F4 (also
// 403, told apart by error.id); the holder's own confirm sells the seat to
// the signed-in user.
func TestF4_HTTPConfirmIsForSignedInCustomers(t *testing.T) {
	w := newWorld(t)
	w.doHold(1, testkit.Alice, t0)
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0+10, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(confirm_hold.Route, httpx.Bind(confirm_hold.Roles, confirm_hold.New(db.New(txn.DB(w.conn))).Handle))
	h := testkit.Serve(mux)
	for _, c := range []struct {
		user, role string
		code       int
		id         string
	}{
		{"", "", http.StatusUnauthorized, "unauthorized"},
		{strconv.FormatInt(testkit.Olga, 10), "organizer", http.StatusForbidden, "forbidden"},
		{strconv.FormatInt(testkit.Ada, 10), "admin", http.StatusForbidden, "forbidden"},
		{strconv.FormatInt(testkit.Bob, 10), "customer", confirm_hold.F4.Status, confirm_hold.F4.ID},
		{strconv.FormatInt(testkit.Alice, 10), "customer", http.StatusBadRequest, "bad_request"}, // sends user itself
	} {
		body := `{"seat_id": 1}`
		if c.id == "bad_request" {
			body = `{"seat_id": 1, "user": 1}`
		}
		w.expectUnchanged(1, func() {
			if rec := testkit.Do(h, http.MethodPost, "/api/holds/confirm", body, c.user, c.role); rec.Code != c.code || testkit.ErrorID(rec) != c.id {
				t.Fatalf("%s %s: %d %s, want %d %s", c.user, c.role, rec.Code, rec.Body, c.code, c.id)
			}
		})
	}
	if rec := testkit.Do(h, http.MethodPost, "/api/holds/confirm", `{"seat_id": 1}`, strconv.FormatInt(testkit.Alice, 10), "customer"); rec.Code != http.StatusCreated {
		t.Fatalf("holder: %d %s", rec.Code, rec.Body)
	}
	if r := testkit.Seat(t, w.conn, 1); r.SoldTo != testkit.Alice || r.SoldAt != t0+10 {
		t.Fatalf("stored %+v", r)
	}
}

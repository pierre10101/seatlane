package checks

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/hold_seat"
	"github.com/pierre10101/seatlane/features/hold_seat/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

func id(u int64) string { return strconv.FormatInt(u, 10) }

// Over HTTP the server sets now (httpx.ClockRule) and the signed-in user
// (httpx.UserRule, from the sign-in hook); the caller can set neither, in the
// body or the query string. Only a signed-in customer may hold a seat: 401
// when not signed in, 403 for an organizer or admin, and nothing is written
// either way. Errors carry the F-ID in error.id.
func TestF1_HTTPHoldIsForSignedInCustomers(t *testing.T) {
	conn := testkit.Open(t, 3)
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(hold_seat.Route, httpx.Bind(hold_seat.Roles, hold_seat.New(db.New(txn.DB(conn))).Handle))
	h := testkit.Serve(mux)
	post := func(user int64, role, target, body string) (int, string, []byte) {
		u := ""
		if user != 0 {
			u = id(user)
		}
		rec := testkit.Do(h, http.MethodPost, target, body, u, role)
		return rec.Code, testkit.ErrorID(rec), rec.Body.Bytes()
	}

	for _, r := range []struct{ target, body string }{
		{"/api/holds", `{"seat_id": 1, "user": 2}`},
		{"/api/holds", `{"seat_id": 1, "User": 2}`},
		{"/api/holds?user=2", `{"seat_id": 1}`},
		{"/api/holds", `{"seat_id": 1, "role": "customer"}`},
		{"/api/holds", `{"seat_id": 1, "now": 1}`},
		{"/api/holds?now=1", `{"seat_id": 1}`},
	} {
		if code, _, body := post(testkit.Alice, "customer", r.target, r.body); code != http.StatusBadRequest {
			t.Fatalf("caller-sent %s %s: %d %s", r.target, r.body, code, body)
		}
	}
	for _, c := range []struct {
		user int64
		role string
		code int
		id   string
	}{
		{0, "", http.StatusUnauthorized, "unauthorized"},
		{testkit.Olga, "organizer", http.StatusForbidden, "forbidden"},
		{testkit.Ada, "admin", http.StatusForbidden, "forbidden"},
		{testkit.Alice, "superuser", http.StatusUnauthorized, "unauthorized"}, // a role AppRoles does not declare: not signed in
	} {
		if code, eid, body := post(c.user, c.role, "/api/holds", `{"seat_id": 1}`); code != c.code || eid != c.id {
			t.Fatalf("%d %s: %d %s, want %d %s", c.user, c.role, code, body, c.code, c.id)
		}
	}
	if r := testkit.Seat(t, conn, 1); r != (testkit.Row{}) {
		t.Fatalf("a refused request wrote: %+v", r)
	}

	code, _, body := post(testkit.Alice, "customer", "/api/holds", `{"seat_id": 1}`)
	var out hold_seat.Output
	if code != http.StatusCreated || json.Unmarshal(body, &out) != nil ||
		out != (hold_seat.Output{SeatID: 1, HeldAt: t0, ExpiresAt: t0 + 600, Now: t0}) {
		t.Fatalf("hold: %d %s", code, body)
	}
	if r := testkit.Seat(t, conn, 1); r.HeldBy != testkit.Alice {
		t.Fatalf("held by %d, want the signed-in user", r.HeldBy)
	}
	// Another customer: F1 with its id.
	if code, eid, body := post(testkit.Bob, "customer", "/api/holds", `{"seat_id": 1}`); code != hold_seat.F1.Status || eid != hold_seat.F1.ID {
		t.Fatalf("second customer: %d %s", code, body)
	}
}

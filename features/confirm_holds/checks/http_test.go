package checks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/confirm_holds"
	"github.com/pierre10101/seatlane/features/confirm_holds/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

// serve binds the route as cmd/server does, behind a test sign-in hook;
// post sends a request as user (0: not signed in) with role.
func serve(t *testing.T) (*world, func(user int64, role, target, body string) *httptest.ResponseRecorder) {
	t.Helper()
	w := newWorld(t)
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0+10, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(confirm_holds.Route, httpx.Bind(confirm_holds.Roles, confirm_holds.New(db.New(txn.DB(w.conn))).Handle))
	h := testkit.Serve(mux)
	post := func(user int64, role, target, body string) *httptest.ResponseRecorder {
		u := ""
		if user != 0 {
			u = strconv.FormatInt(user, 10)
		}
		return testkit.Do(h, http.MethodPost, target, body, u, role)
	}
	return w, post
}

// D10 over HTTP: an empty list, more than 20 ids, a repeated id, a null or
// non-whole id, no list, or a caller-sent user, role or time is HTTP 400
// bad_request; the action does not run and nothing is written.
func TestHTTPListInputIsChecked(t *testing.T) {
	w, post := serve(t)
	w.doHold(testkit.Alice, t0, 1, 2)
	ids21 := strings.TrimSuffix(strings.Repeat("1,", 21), ",")
	ids21distinct := "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21"
	const route = "/api/holds/confirm-all"
	w.expectNothingWritten(func() {
		for _, r := range []struct{ target, body string }{
			{route, `{"seat_ids": []}`},
			{route, `{"seat_ids": [` + ids21 + `]}`},
			{route, `{"seat_ids": [` + ids21distinct + `]}`},
			{route, `{"seat_ids": [1, 2, 1]}`},
			{route, `{"seat_ids": [1, "2"]}`},
			{route, `{"seat_ids": [1, 2.5]}`},
			{route, `{"seat_ids": [1, null]}`},
			{route, `{"seat_ids": null}`},
			{route, `{}`},
			{route, `{"seat_ids": [1], "user": 2}`},
			{route, `{"seat_ids": [1], "role": "customer"}`},
			{route, `{"seat_ids": [1], "now": 1}`},
			{route + "?user=2", `{"seat_ids": [1]}`},
		} {
			if rec := post(testkit.Alice, "customer", r.target, r.body); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"bad_request"`) {
				t.Fatalf("%s %s: %d %s", r.target, r.body, rec.Code, rec.Body)
			}
		}
	})
	rec := post(testkit.Alice, "customer", route, `{"seat_ids": [2, 1]}`)
	var out confirm_holds.Output
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != http.StatusCreated || out != (confirm_holds.Output{Confirmed: 2, SoldAt: t0 + 10, Now: t0 + 10}) {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body)
	}
}

// Over HTTP: not signed in is 401, an organizer or admin 403 (the runtime's
// "unauthorized" and "forbidden", before the action runs), a foreign seat
// F13 (409), an expired hold F2 (410); error.id carries the F-ID or the
// runtime's id, and nothing is written in any of these.
func TestF2F13_HTTPStatuses(t *testing.T) {
	w, post := serve(t)
	w.doHold(testkit.Alice, t0, 1)
	w.doHold(testkit.Alice, t0-600, 2)
	w.doHold(testkit.Bob, t0, 3)
	for _, c := range []struct {
		user   int64
		role   string
		body   string
		status int
		id     string
	}{
		{0, "", `{"seat_ids": [1]}`, http.StatusUnauthorized, "unauthorized"},
		{testkit.Olga, "organizer", `{"seat_ids": [1]}`, http.StatusForbidden, "forbidden"},
		{testkit.Ada, "admin", `{"seat_ids": [1]}`, http.StatusForbidden, "forbidden"},
		{testkit.Alice, "customer", `{"seat_ids": [1, 3]}`, confirm_holds.F13.Status, confirm_holds.F13.ID},
		{testkit.Alice, "customer", `{"seat_ids": [1, 2]}`, confirm_holds.F2.Status, confirm_holds.F2.ID},
	} {
		w.expectNothingWritten(func() {
			rec := post(c.user, c.role, "/api/holds/confirm-all", c.body)
			if rec.Code != c.status || testkit.ErrorID(rec) != c.id {
				t.Fatalf("%d %s %s: %d %s, want %d %s", c.user, c.role, c.body, rec.Code, rec.Body, c.status, c.id)
			}
		})
	}
}

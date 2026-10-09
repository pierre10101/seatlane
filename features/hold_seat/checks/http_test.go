package checks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/hold_seat"
	"github.com/pierre10101/seatlane/features/hold_seat/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

// Over HTTP the server sets now (httpx.ClockRule) and session (the cookie
// bridge_session, httpx.SessionRule); the caller can set neither, in the body
// or the query string. Without a valid cookie the session is the empty text: F8. Errors
// carry the F-ID in error.id.
func TestF1_HTTPHoldUsesServerTimeAndCookieSession(t *testing.T) {
	conn := testkit.Open(t, 3)
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(hold_seat.Route, httpx.Bind(hold_seat.New(db.New(txn.DB(conn))).Handle))
	post := func(cookie, target, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: httpx.SessionCookie, Value: cookie})
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	for _, r := range []struct{ target, body string }{
		{"/api/holds", `{"seat_id": 1, "session": 7}`},
		{"/api/holds", `{"seat_id": 1, "Session": 7}`},
		{"/api/holds?session=7", `{"seat_id": 1}`},
		{"/api/holds", `{"seat_id": 1, "now": 1}`},
		{"/api/holds?now=1", `{"seat_id": 1}`},
	} {
		if rec := post(testkit.Alice, r.target, r.body); rec.Code != http.StatusBadRequest {
			t.Fatalf("caller-sent %s %s: %d %s", r.target, r.body, rec.Code, rec.Body)
		}
	}
	if r := testkit.Seat(t, conn, 1); r.HeldBy != "" {
		t.Fatalf("a refused request held the seat: %+v", r)
	}
	for _, cookie := range []string{"", "abc!", strings.Repeat("x", 129)} {
		rec := post(cookie, "/api/holds", `{"seat_id": 1}`)
		var body httpx.ErrorBody
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != hold_seat.F8.Status || body.Error.ID != hold_seat.F8.ID {
			t.Fatalf("cookie %q: %d %s", cookie, rec.Code, rec.Body)
		}
	}
	rec := post(testkit.Alice, "/api/holds", `{"seat_id": 1}`)
	var out hold_seat.Output
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &out) != nil ||
		out != (hold_seat.Output{SeatID: 1, HeldAt: t0, ExpiresAt: t0 + 600, Now: t0}) {
		t.Fatalf("hold: %d %s", rec.Code, rec.Body)
	}
	// Another visitor's session: F1 with its id.
	rec = post(testkit.Bob, "/api/holds", `{"seat_id": 1}`)
	var body httpx.ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != hold_seat.F1.Status || body.Error.ID != hold_seat.F1.ID {
		t.Fatalf("second visitor: %d %s", rec.Code, rec.Body)
	}
}

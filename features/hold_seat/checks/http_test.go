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
	"github.com/pierre10101/seatlane/internal/session"
	"github.com/pierre10101/seatlane/internal/testkit"
)

// Over HTTP the server sets now (httpx.ClockRule) and session (the cookie,
// internal/session); the caller can set neither. Errors carry the F-ID in
// error.id.
func TestF1_HTTPHoldUsesServerTimeAndCookieSession(t *testing.T) {
	conn := testkit.Open(t, 3)
	old := httpx.Now
	httpx.Now = func() time.Time { return time.Unix(t0, 0) }
	t.Cleanup(func() { httpx.Now = old })
	mux := http.NewServeMux()
	mux.Handle(hold_seat.Route, httpx.Bind(hold_seat.New(db.New(txn.DB(conn))).Handle))
	h := session.Wrap(mux)
	post := func(cookie, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/holds", strings.NewReader(body))
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: session.Cookie, Value: cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := post("1001", `{"seat_id": 1, "session": 7}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("caller-sent session: %d %s", rec.Code, rec.Body)
	}
	if rec := post("1001", `{"seat_id": 1, "now": 1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("caller-sent now: %d %s", rec.Code, rec.Body)
	}
	rec := post("1001", `{"seat_id": 1}`)
	var out hold_seat.Output
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &out) != nil ||
		out != (hold_seat.Output{SeatID: 1, HeldAt: t0, ExpiresAt: t0 + 600, Now: t0}) {
		t.Fatalf("hold: %d %s", rec.Code, rec.Body)
	}
	// A visitor without a cookie gets a fresh session, and F1 with its id.
	rec = post("", `{"seat_id": 1}`)
	var body httpx.ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != hold_seat.F1.Status || body.Error.ID != hold_seat.F1.ID {
		t.Fatalf("second visitor: %d %s", rec.Code, rec.Body)
	}
	if c := rec.Result().Cookies(); len(c) != 1 || c[0].Name != session.Cookie || !c[0].HttpOnly {
		t.Fatalf("no session cookie minted: %v", c)
	}
}

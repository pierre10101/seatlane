package checks

import (
	"net/http"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/list_events"
	"github.com/pierre10101/seatlane/features/list_events/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

// Over HTTP the event list is public: signed out and every role get 200;
// F11 answers with its id for anyone.
func TestF11_HTTPListEventsIsPublic(t *testing.T) {
	_, conn := newAction(t)
	mux := http.NewServeMux()
	mux.Handle(list_events.Route, httpx.Bind(list_events.Roles, list_events.New(db.New(txn.DB(conn))).Handle))
	h := testkit.Serve(mux)
	for _, as := range [][2]string{{"", ""}, {"1", "customer"}, {"3", "organizer"}, {"4", "admin"}} {
		if rec := testkit.Do(h, http.MethodGet, "/api/events", "", as[0], as[1]); rec.Code != http.StatusOK {
			t.Fatalf("%v: %d %s", as, rec.Code, rec.Body)
		}
		if rec := testkit.Do(h, http.MethodGet, "/api/events?limit=101", "", as[0], as[1]); rec.Code != list_events.F11.Status || testkit.ErrorID(rec) != list_events.F11.ID {
			t.Fatalf("%v limit 101: %d %s", as, rec.Code, rec.Body)
		}
	}
}

package main

import (
	"database/sql"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/confirm_hold"
	confirmdb "github.com/pierre10101/seatlane/features/confirm_hold/db"
	"github.com/pierre10101/seatlane/features/confirm_holds"
	confirmalldb "github.com/pierre10101/seatlane/features/confirm_holds/db"
	"github.com/pierre10101/seatlane/features/hold_seat"
	holddb "github.com/pierre10101/seatlane/features/hold_seat/db"
	"github.com/pierre10101/seatlane/features/list_events"
	eventsdb "github.com/pierre10101/seatlane/features/list_events/db"
	"github.com/pierre10101/seatlane/features/list_my_holds"
	myholdsdb "github.com/pierre10101/seatlane/features/list_my_holds/db"
	"github.com/pierre10101/seatlane/features/me"
	medb "github.com/pierre10101/seatlane/features/me/db"
	"github.com/pierre10101/seatlane/features/release_hold"
	releasedb "github.com/pierre10101/seatlane/features/release_hold/db"
	"github.com/pierre10101/seatlane/features/show_seat_map"
	mapdb "github.com/pierre10101/seatlane/features/show_seat_map/db"
	"github.com/pierre10101/seatlane/internal/auth"
)

// AppRoles is every role a signed-in user of Seatlane can have (bridge-en
// A2). An action's Roles may only list these; bridge-en refuses a typo, and
// httpx.Identify treats any other role from the sign-in hook as signed out.
// Sign-up creates customers; organizers and admins are created by hand for
// now (schema.sql users.role allows exactly these three).
var AppRoles = httpx.AppRoles("customer", "organizer", "admin")

// API wires every slice. One line per feature: Route constant -> Handle,
// bound with the slice's own Roles, so httpx.Bind answers 401/403 before the
// action runs (A3). Queries go through txn.DB, so httpx.Bind runs each call
// in one transaction. The sign-up, sign-in and sign-out endpoints are
// Seatlane's own code (internal/auth, outside bridge-en), and so is the
// sign-in hook: identity reads the seatlane_auth cookie and looks its
// session up; httpx.Identify hands the user and role to every Bind.
func API(db *sql.DB, accounts *auth.Service) http.Handler {
	identity := httpx.Identity(accounts.Identify)
	mux := http.NewServeMux()
	mux.Handle(list_events.Route, httpx.Bind(list_events.Roles, list_events.New(eventsdb.New(txn.DB(db))).Handle))
	mux.Handle(show_seat_map.Route, httpx.Bind(show_seat_map.Roles, show_seat_map.New(mapdb.New(txn.DB(db))).Handle))
	mux.Handle(list_my_holds.Route, httpx.Bind(list_my_holds.Roles, list_my_holds.New(myholdsdb.New(txn.DB(db))).Handle))
	mux.Handle(hold_seat.Route, httpx.Bind(hold_seat.Roles, hold_seat.New(holddb.New(txn.DB(db))).Handle))
	mux.Handle(confirm_hold.Route, httpx.Bind(confirm_hold.Roles, confirm_hold.New(confirmdb.New(txn.DB(db))).Handle))
	mux.Handle(confirm_holds.Route, httpx.Bind(confirm_holds.Roles, confirm_holds.New(confirmalldb.New(txn.DB(db))).Handle))
	mux.Handle(release_hold.Route, httpx.Bind(release_hold.Roles, release_hold.New(releasedb.New(txn.DB(db))).Handle))
	mux.Handle(me.Route, httpx.Bind(me.Roles, me.New(medb.New(txn.DB(db))).Handle))
	accounts.Mount(mux)
	return httpx.Identify(AppRoles, identity, mux)
}

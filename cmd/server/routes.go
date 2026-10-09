package main

import (
	"database/sql"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/confirm_hold"
	confirmdb "github.com/pierre10101/seatlane/features/confirm_hold/db"
	"github.com/pierre10101/seatlane/features/hold_seat"
	holddb "github.com/pierre10101/seatlane/features/hold_seat/db"
	"github.com/pierre10101/seatlane/features/list_events"
	eventsdb "github.com/pierre10101/seatlane/features/list_events/db"
	"github.com/pierre10101/seatlane/features/list_my_holds"
	myholdsdb "github.com/pierre10101/seatlane/features/list_my_holds/db"
	"github.com/pierre10101/seatlane/features/release_hold"
	releasedb "github.com/pierre10101/seatlane/features/release_hold/db"
	"github.com/pierre10101/seatlane/features/show_seat_map"
	mapdb "github.com/pierre10101/seatlane/features/show_seat_map/db"
)

// API wires every slice. One line per feature: Route constant -> Handle.
// Queries go through txn.DB, so httpx.Bind runs each call in one transaction.
func API(db *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(list_events.Route, httpx.Bind(list_events.New(eventsdb.New(txn.DB(db))).Handle))
	mux.Handle(show_seat_map.Route, httpx.Bind(show_seat_map.New(mapdb.New(txn.DB(db))).Handle))
	mux.Handle(list_my_holds.Route, httpx.Bind(list_my_holds.New(myholdsdb.New(txn.DB(db))).Handle))
	mux.Handle(hold_seat.Route, httpx.Bind(hold_seat.New(holddb.New(txn.DB(db))).Handle))
	mux.Handle(confirm_hold.Route, httpx.Bind(confirm_hold.New(confirmdb.New(txn.DB(db))).Handle))
	mux.Handle(release_hold.Route, httpx.Bind(release_hold.New(releasedb.New(txn.DB(db))).Handle))
	return mux
}

// Command server runs Seatlane: the JSON API under /api/ and, when built,
// the web app from web/dist.
//
//	go run ./cmd/server -db seatlane.db -addr :8080 -web web/dist
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
	"github.com/pierre10101/seatlane/internal/devclock"
	"github.com/pierre10101/seatlane/internal/seed"
	"github.com/pierre10101/seatlane/internal/session"
	"github.com/pierre10101/seatlane/internal/web"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	path := flag.String("db", "seatlane.db", "SQLite database file")
	dist := flag.String("web", "web/dist", "built web app to serve (skipped when missing)")
	doSeed := flag.Bool("seed", true, "seed demo events into an empty database")
	devClock := flag.Bool("dev-clock", false, "enable POST /__dev/advance?seconds=N to move the server clock forward (development only)")
	flag.Parse()

	ctx := context.Background()
	db, err := store.Open(ctx, *path, seatlane.Schema)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if *doSeed {
		if err := seed.IfEmpty(ctx, db); err != nil {
			log.Fatal(err)
		}
	}

	h := Handler(API(db), *dist)
	if *devClock {
		devclock.Install()
		mux := http.NewServeMux()
		mux.Handle("/__dev/advance", devclock.Handler())
		mux.Handle("/", h)
		h = mux
		log.Printf("dev clock enabled: POST /__dev/advance?seconds=N")
	}
	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, h))
}

// Handler mounts the API (behind the session cookie) and the web app.
func Handler(api http.Handler, dist string) http.Handler {
	root := http.NewServeMux()
	root.Handle("/api/", session.Wrap(api))
	if st, err := os.Stat(dist); err == nil && st.IsDir() {
		root.Handle("/", web.Handler(dist))
	} else {
		log.Printf("no web app at %s; serving the API only (run make build)", dist)
	}
	return root
}

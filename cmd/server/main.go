// Command server runs Seatlane: the JSON API under /api/ and, when built,
// the web app from web/dist.
//
//	go run ./cmd/server -db seatlane.db -addr :8080 -web web/dist
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/pierre10101/go-ai-bridge/runtime/store"
	"github.com/pierre10101/seatlane"
	"github.com/pierre10101/seatlane/internal/auth"
	"github.com/pierre10101/seatlane/internal/devclock"
	"github.com/pierre10101/seatlane/internal/seed"
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

	accounts := auth.New(db, csrfKey())
	h := Handler(API(db, accounts), accounts, *dist)
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

// csrfKey is the CSRF HMAC key: SEATLANE_CSRF_KEY (64 hex characters) or,
// without it, 32 random bytes for this process (tokens then stop working
// after a restart; the web app gets F19 with a fresh cookie and retries).
func csrfKey() []byte {
	if v := os.Getenv("SEATLANE_CSRF_KEY"); v != "" {
		k, err := hex.DecodeString(v)
		if err != nil || len(k) != 32 {
			log.Fatal("SEATLANE_CSRF_KEY must be 64 hex characters (32 bytes)")
		}
		return k
	}
	return auth.NewKey()
}

// Handler mounts the API and the web app. Every request under /api/ passes
// the CSRF check first (accounts.CSRF: a state-changing request without a
// valid X-CSRF-Token is F19 and never reaches a feature); every response
// that serves the HTML sets the CSRF cookie. Who is signed in is decided
// inside api (httpx.Identify with accounts.Identify); nothing is injected.
func Handler(api http.Handler, accounts *auth.Service, dist string) http.Handler {
	root := http.NewServeMux()
	root.Handle("/api/", accounts.CSRF(api))
	if st, err := os.Stat(dist); err == nil && st.IsDir() {
		root.Handle("/", web.Handler(dist, accounts.EnsureCSRFCookie))
	} else {
		log.Printf("no web app at %s; serving the API only (run make build)", dist)
	}
	return root
}

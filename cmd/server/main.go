// Command server runs Seatlane: the JSON API under /api/ and, when built,
// the web app from web/dist.
//
//	go run ./cmd/server -db seatlane.db -addr :8080 -web web/dist
//
// With -seed-dev-accounts (and SEATLANE_DEV=1; `make seed`) it creates the
// dev accounts, prints their password and exits instead of serving.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pierre10101/seatlane/internal/auth"
	"github.com/pierre10101/seatlane/internal/dbopen"
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
	seedDev := flag.Bool("seed-dev-accounts", false, "create the dev accounts (organizer@example.test, admin@example.test), print their password and exit; refuses unless SEATLANE_DEV=1 (local development only)")
	flag.Parse()

	var devPassword string
	if *seedDev {
		p, err := seed.DevPassword(os.Getenv)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		devPassword = p
	}

	ctx := context.Background()
	db, err := dbopen.Open(ctx, *path)
	if out := (*dbopen.OutdatedError)(nil); errors.As(err, &out) {
		fmt.Fprintf(os.Stderr, "%s\n  (%s; the schema changed and there are no migrations, so start from a fresh database)\n", out, out.Reason)
		os.Exit(1)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if *doSeed {
		if err := seed.IfEmpty(ctx, db); err != nil {
			log.Fatal(err)
		}
	}
	if *seedDev {
		if err := seed.LoadDevAccounts(ctx, db, devPassword, auth.New(db, nil).BcryptCost, time.Now().Unix()); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Dev accounts in %s (local use only; running this again resets their password):\n", *path)
		for _, a := range seed.DevAccounts {
			fmt.Printf("  %-24s %-10s password: %s\n", a.Email, a.Role, devPassword)
		}
		return
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

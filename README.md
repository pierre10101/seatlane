# Seatlane

**Pick a seat, hold it for ten minutes, confirm.** Seatlane is a small
seat-booking app for live shows where two people can never hold the same
seat, a hold that runs out frees the seat for the next person, and every one
of those rules can be reviewed in plain English before it ships.

The backend is Go + [sqlc](https://sqlc.dev) on SQLite, written as feature
slices that [bridge-en](https://github.com/pierre10101/go-ai-bridge) compiles
to controlled English. The UI is a thin React app (Vite, TypeScript,
Tailwind, shadcn/ui, lucide-react, framer-motion) that draws exactly what the
server says.

| Light | Dark |
|---|---|
| ![Event list, light](docs/screenshots/events-light.png) | ![Event list, dark](docs/screenshots/events-dark.png) |
| ![Seat map with my holds and countdowns, light](docs/screenshots/seatmap-light.png) | ![Seat map with my holds and countdowns, dark](docs/screenshots/seatmap-dark.png) |
| ![F1 toast: the seat is already held, light](docs/screenshots/toast-f1-light.png) | ![F1 toast: the seat is already held, dark](docs/screenshots/toast-f1-dark.png) |
| ![Booking confirmed](docs/screenshots/success-light.png) | ![A hold that expired](docs/screenshots/expired-light.png) |

<p align="center">
  <img src="docs/screenshots/mobile-seatmap-light.png" alt="Seat map on a phone, light" width="260">
  <img src="docs/screenshots/mobile-seatmap-dark.png" alt="Seat map on a phone, dark" width="260">
</p>

## How it works

- **One slice per action** under `features/`: `list_events`, `show_seat_map`,
  `list_my_holds`, `hold_seat`, `confirm_hold`, `release_hold`. Each has an
  `intent.md` (why, inputs, outputs, failure cases written first), plain SQL in
  `queries/`, sqlc code in `db/`, the action in `action.go`, one check per
  failure ID in `checks/`, and its English in `<slice>.en`.
- **Check and write in one statement.** Holding, confirming and releasing are
  each a single conditional `UPDATE` ("only if, at that moment, the seat is
  not sold and nobody holds it or its hold has expired") whose changed-row
  count must be exactly one. Reads after it only explain why it changed
  nothing (F1, F6, ...). Check-then-write is refused by bridge-en.
- **The server owns time and identity.** `now` (`clock:"now"`) and `session`
  (`server:"session"`) are set by the bridge-en runtime from the server clock
  and the `bridge_session` cookie; a request that sends either is a 400. A
  hold is active while `expires_at` is later than now: a hold taken exactly
  600 seconds ago has expired, one taken 599 seconds ago has not.
- **Anonymous sessions.** The session is 128 random bits from `crypto/rand`,
  written as 26 base32 characters. The server sets the cookie on every
  response that serves the web app's HTML, and on any API call without a valid
  one as a fallback (`internal/session`); it is `HttpOnly`, `SameSite=Lax`,
  `Path=/`, and `Secure` over TLS. `held_by` and `sold_to` store it as TEXT,
  with `''` meaning nobody.
- **Errors carry a stable ID.** Every failure is
  `{"error": {"id": "F1", "message": "..."}}`; the catalogue is in
  [docs/failures.md](docs/failures.md). The UI maps `error.id` to friendly
  copy and never reads the message.

### A thin UI

All seat and hold rules live in Go. The web app:

- draws each seat from the server's five flags (`available`, `held_by_me`,
  `held_by_other`, `sold_to_me`, `sold_to_other`) and never decides on its own
  whether a seat is free or a hold has expired;
- counts down from `expires_at - now` as the server reported it (from the hold
  answer and `GET /api/events/{id}/holds`), minus the time since that answer
  arrived; it re-reads the server when a countdown reaches zero and every 4
  seconds, and only shows a hold as expired when the server's `active` flag
  says so;
- shows a pending state the moment you click a seat, then reconciles with the
  server's answer; every failure ID gets a toast with its own copy.

The seat map is keyboard-navigable (arrow keys, Home/End, Enter), has
hover/focus tooltips with the seat label and price in rand, and announces
holds, releases, bookings, expiries and errors in an `aria-live` region.

## Where the English lives

Each slice's review copy sits next to its code:

```
features/hold_seat/hold_seat.en          features/confirm_hold/confirm_hold.en
features/release_hold/release_hold.en    features/show_seat_map/show_seat_map.en
features/list_my_holds/list_my_holds.en  features/list_events/list_events.en
```

They are generated (`bridge-en -write`), never edited by hand, and checked in
CI (`bridge-en -check features/*/`): the check fails if the code and its
English drift apart, if a slice uses a construct outside the
[rulebook](https://github.com/pierre10101/go-ai-bridge/blob/v0.1.2/RULEBOOK.md),
or if the failure IDs in `intent.md`, `action.go` and `checks/` differ. Read a
`.en` diff in a pull request the way you would read the code.

## Run it

Requirements: Go 1.24, Node 20.19+ (22 recommended), and for `make check` the
bridge-en binary of the version `go.mod` pins (`make tools`).

```sh
make tools   # go install github.com/pierre10101/go-ai-bridge/cmd/bridge-en@<pinned version>
make dev     # Go API with the dev clock on :8080, Vite on http://localhost:5173 (proxies /api)
make build   # web/dist + bin/seatlane
make run     # build, then serve the API and web/dist on http://localhost:8080
make check   # bridge-en -check features/*/
make test    # go vet + go test ./...
```

The server seeds three demo events into an empty database. With `-dev-clock`
(on in `make dev`), `POST /__dev/advance?seconds=N` moves the server clock
forward, so you can watch a hold expire without waiting ten minutes:

```sh
curl -X POST 'http://localhost:8080/__dev/advance?seconds=600'
```

After changing `schema.sql` or a query, run `make generate` (sqlc + the
`.en` files) and review the diff. An existing local database from before the
TEXT session columns must be deleted (`rm seatlane.db`); the server seeds a
fresh one.

## API

| Route | Slice | Failure IDs |
|---|---|---|
| `GET /api/events` | list_events | F11, F12 |
| `GET /api/events/{id}/seats` | show_seat_map | F8, F10, F11, F12 |
| `GET /api/events/{id}/holds` | list_my_holds | F8, F11, F12 |
| `POST /api/holds` `{"seat_id"}` | hold_seat | F1, F6, F7, F8 |
| `POST /api/holds/confirm` `{"seat_id"}` | confirm_hold | F2, F3, F4, F5, F6, F7, F8 |
| `POST /api/holds/release` `{"seat_id"}` | release_hold | F5, F6, F7, F8, F9 |

## Layout

```
cmd/server/        main (store.Open, seed, -dev-clock) and routes.go (one httpx.Bind line per slice)
features/<slice>/  intent.md, action.go, queries/, db/, checks/, <slice>.en
internal/domain/   value objects and pure seat rules (rendered into the English)
internal/session/  issues the bridge_session cookie
internal/web/      serves web/dist with the SPA fallback (and the cookie)
web/               the React app
docs/              failures.md, screenshots/
```

## License

MIT. See [LICENSE](LICENSE).

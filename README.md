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
| ![Confirm all 2 seats: booked together, or none are, light](docs/screenshots/confirm-all-light.png) | ![Confirm all 2 seats: booked together, or none are, dark](docs/screenshots/confirm-all-dark.png) |
| ![Confirm all with one expired hold: F2 toast, nothing booked, the other seat still held, light](docs/screenshots/confirm-all-f2-light.png) | ![Confirm all with one expired hold: F2 toast, nothing booked, the other seat still held, dark](docs/screenshots/confirm-all-f2-dark.png) |

<p align="center">
  <img src="docs/screenshots/mobile-seatmap-light.png" alt="Seat map on a phone, light" width="260">
  <img src="docs/screenshots/mobile-seatmap-dark.png" alt="Seat map on a phone, dark" width="260">
  <img src="docs/screenshots/mobile-release-confirm-dark.png" alt="Tapping your own held seat on a phone asks before releasing it" width="260">
</p>

## How it works

- **One slice per action** under `features/`: `list_events`, `show_seat_map`,
  `list_my_holds`, `hold_seat`, `confirm_hold`, `confirm_holds`, `release_hold`, `me`. Each has an
  `intent.md` (why, inputs, outputs, failure cases written first), plain SQL in
  `queries/`, sqlc code in `db/`, the action in `action.go`, one check per
  failure ID in `checks/`, and its English in `<slice>.en`.
- **Check and write in one statement.** Holding, confirming and releasing are
  each a single conditional `UPDATE` ("only if, at that moment, the seat is
  not sold and nobody holds it or its hold has expired") whose changed-row
  count must be exactly one. Reads after it only explain why it changed
  nothing (F1, F6, ...). Check-then-write is refused by bridge-en.
- **Several seats: all or none.** `confirm_holds` takes `seat_ids` (1 to 20
  ids, no duplicates; anything else is a 400) and sells them with one
  conditional `UPDATE ... AND id IN (sqlc.slice(seat_ids))`; unless it changed
  exactly one row per listed seat, the transaction rolls back and no seat is
  sold (F2 if one of the holds has expired, F13 if a seat is not held by you).
- **The server owns time and identity.** `now` (`clock:"now"`), `user`
  (`server:"user"`) and `role` (`server:"role"`) are set by the bridge-en
  runtime from the server clock and the signed-in session; a request that
  sends any of them is a 400. A hold is active while `expires_at` is later
  than now: a hold taken exactly 600 seconds ago has expired, one taken 599
  seconds ago has not.
- **Roles, deny by default.** The app's roles are declared once in
  `cmd/server/routes.go` (`httpx.AppRoles("customer", "organizer", "admin")`)
  and every slice declares `var Roles`: `list_events` and `show_seat_map` are
  `httpx.Public`; holding, confirming, releasing and listing your holds are
  `httpx.Roles("customer")`; `me` is for every role. `httpx.Bind` answers 401
  (`unauthorized`) when nobody is signed in and 403 (`forbidden`) for another
  role, before the action reads anything. Organizers and admins exist as
  roles, but no slice grants them more yet; ownership rules wait for
  bridge-en rule A4.
- **Accounts and sessions** (`internal/auth`, app code: see
  [docs/bridge-en-gaps.md](docs/bridge-en-gaps.md) for why these are not
  slices). Sign-up stores a bcrypt hash (cost 12) of a 10-72 byte password;
  the email is trimmed and lower-cased, and a duplicate is F16 even under
  concurrent sign-ups (`INSERT ... ON CONFLICT DO NOTHING`). Sign-in answers
  F17 for an unknown email and for a wrong password alike (an unknown email is
  checked against a dummy hash, so it takes as long). A session is 32 random
  bytes in the `seatlane_auth` cookie (`HttpOnly`, `SameSite=Lax`, `Path=/`,
  `Secure` over TLS, 7 days); the database stores only its SHA-256.
  `httpx.Identify(AppRoles, identity, mux)` turns it into the user id and
  role through the `httpx.Identity` hook. `held_by` and `sold_to` store the
  user id, with `0` meaning nobody.
- **CSRF.** Every `POST`/`PUT`/`PATCH`/`DELETE` under `/api/` must send
  `X-CSRF-Token` equal to the `seatlane_csrf` cookie (double submit). The
  token is `nonce.HMAC-SHA256(key, nonce | sha256(session))`, so it is only
  valid with the sign-in session it was issued for, and a cross-site `Origin`
  or `Sec-Fetch-Site` is refused too. Anything else is F19 (403), with a
  fresh cookie so the web app can retry once. The key comes from
  `SEATLANE_CSRF_KEY` (64 hex characters) or is random per process.
- **Sign-in rate limit.** Two caps, each per 15-minute window: at most 5
  failed sign-ins per client IP + email, and at most 20 failed sign-ins per
  client IP across all emails. Once either is reached, the next attempt is
  F18 (429, `Retry-After`) without checking the password. There is no cap per
  email across IPs, so nobody can lock an account's owner out from elsewhere.
  The rules are pure functions, `auth.ReserveBoth(pair, ip, now)` and
  `auth.Refund`, and `now` is passed in (`httpx.Now()`, which the dev clock
  replaces). Each attempt is counted against both caps inside the same
  locked transaction that reads them, before the password is checked, so
  parallel guesses cannot get past either limit. A successful sign-in
  clears its IP + email count and gives back its one in the IP count, so
  only failures use up the IP's 20.
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

On screens under 640px every seat is a tap target of at least 40px; the map
scrolls sideways inside its card and has +/- buttons for bigger seats (40,
48, 56px). Tapping your own held seat on a touch screen asks "Release …?"
before giving it back; a mouse click or Enter releases at once.

**All seats together, or none.** "Confirm all N seats" sends one
`confirm_holds` call with every seat shown in the panel: "All N seats are
booked together, or none are." If any of them cannot be sold (a hold ran out,
F2; a seat is no longer yours, F13), nothing is booked, your other holds stay
as they were, and a toast says which failure it was. Each seat still has its
own Confirm button, which books just that seat (`confirm_hold`).

The seat map is keyboard-navigable (arrow keys, Home/End, Enter), has
hover/focus tooltips with the seat label and price in rand, and announces
holds, releases, bookings, expiries and errors in an `aria-live` region.

## Where the English lives

Each slice's review copy sits next to its code:

```
features/hold_seat/hold_seat.en          features/confirm_hold/confirm_hold.en
features/release_hold/release_hold.en    features/show_seat_map/show_seat_map.en
features/list_my_holds/list_my_holds.en  features/list_events/list_events.en
features/confirm_holds/confirm_holds.en  features/me/me.en
```

Sign-up, sign-in and sign-out have no `.en`: they are not bridge-en slices.
Their intent is in `internal/auth/{sign_up,sign_in,sign_out,csrf}/intent.md`,
and `internal/auth/intent_test.go` cross-checks those F-IDs against the code
and the checks.

They are generated (`bridge-en -write`), never edited by hand, and checked in
CI (`bridge-en -check features/*/`): the check fails if the code and its
English drift apart, if a slice uses a construct outside the
[rulebook](https://github.com/pierre10101/go-ai-bridge/blob/v0.3.0/RULEBOOK.md),
or if the failure IDs in `intent.md`, `action.go` and `checks/` differ. Each
`intent.md` lists its failure cases under exactly one `## Failure cases`
heading, one `- F<n>: <text>` line each. Read a `.en` diff in a pull request
the way you would read the code: on every pull request, CI posts one comment
(updated on each push) with each changed feature's intent next to its English
diff.

Agents (and people) changing the app follow [AGENTS.md](AGENTS.md), written
by `bridge-en init`; `CLAUDE.md`, `GEMINI.md`, `.cursor/rules/bridge-en.mdc`
and `.github/copilot-instructions.md` point to it.

## Run it locally

### Prerequisites

- Go 1.24 and Node 20.19+ (22 recommended) with npm.
- Optional: the `sqlite3` command-line shell to look inside the database
  (`brew install sqlite`, `sudo apt install sqlite3`). The server itself
  needs no SQLite install (it uses a pure Go driver).
- Only for `make check`: the bridge-en binary of the version `go.mod` pins
  (`make tools`).

### Start it

Either one server that serves the built web app:

```sh
make build                                  # web/dist + bin/seatlane
./bin/seatlane -db seatlane.db -web web/dist # or: make run (build, then this)
```

and open **http://localhost:8080**. Or, while working on the code, hot reload:

```sh
make dev   # Go API with the dev clock on :8080 + Vite on http://localhost:5173 (proxies /api)
```

and open http://localhost:5173 (port 8080 then answers the API only).

The first run creates `seatlane.db` in the current directory (`-db` picks
another file) with the schema and three demo events. Later runs reuse it.
Other flags: `-addr :8080`, `-web ''` (API only), `-seed=false` (no demo
events), `-dev-clock` (on in `make dev`): `POST /__dev/advance?seconds=N`
moves the server clock forward, so you can watch a hold expire without
waiting ten minutes:

```sh
curl -X POST 'http://localhost:8080/__dev/advance?seconds=600'
```

Other targets: `make check` (bridge-en -check), `make test` (go vet + go
test), `make generate` (after changing `schema.sql` or a query: sqlc and the
`.en` files; review the diff).

### Accounts: sign up, sign in, sign out

There is **no default user**. Click **Sign up** (top right, or
`/sign-up`), enter an email and a password of **10 to 72 bytes**, and you
are signed in as a new **customer**. Every sign-up is a customer; there is
no way to pick a role in the app. **Sign in** (`/sign-in`) takes the same
email and password (the email is trimmed and lower-cased, so
`Ana@Example.com ` and `ana@example.com` are one account); a wrong password
and an unknown email get the same answer (F17). **Sign out** is in the
header. A sign-in lasts 7 days.

| Role | What it can do now |
|---|---|
| (signed out) | browse events and seat maps |
| `customer` | browse; hold seats (10 minutes), confirm one or all held seats, release a hold, see own holds; `GET /api/me` |
| `organizer` | browse and `GET /api/me`; holding and booking answer 403. No organizer screens yet. |
| `admin` | the same as organizer for now. No admin screens yet. |

To make an account an organizer or admin, change its role in the database
(no restart needed; it applies from the next request):

```sh
sqlite3 seatlane.db "UPDATE users SET role = 'organizer' WHERE email = 'olga@example.com';"
sqlite3 seatlane.db "UPDATE users SET role = 'admin' WHERE email = 'olga@example.com';"
```

### Dev accounts (local only)

`make seed` creates (or resets) two accounts in `seatlane.db` with one
password, printed once:

```
$ make seed
Dev accounts in seatlane.db (local use only; running this again resets their password):
  organizer@example.test   organizer  password: <16 random characters>
  admin@example.test       admin      password: <16 random characters>
```

It runs `SEATLANE_DEV=1 go run ./cmd/server -db seatlane.db
-seed-dev-accounts`, which seeds and exits without serving. The server
refuses `-seed-dev-accounts` unless `SEATLANE_DEV=1` is set, so it cannot
happen by accident against a real database; never set it in a deployment.
`SEATLANE_DEV_PASSWORD=...` (10 to 72 bytes) picks the password instead of
a random one; `make seed DB=other.db` seeds another file. Customers are not
seeded: sign up in the app. `example.test` is a reserved domain, so these
addresses reach nobody.

### Look inside the database

```sh
sqlite3 seatlane.db
sqlite> .tables
auth_sessions     events            seats             sign_in_attempts  users
sqlite> SELECT id, email, role FROM users;
sqlite> SELECT id, row_label, seat_number, held_by, expires_at, sold_to FROM seats WHERE held_by != 0 OR sold_to != 0;
sqlite> .quit
```

`held_by` and `sold_to` are a `users.id` (0 means nobody); times are unix
seconds. Passwords are bcrypt hashes and `auth_sessions` stores only a
SHA-256 of each session token, so neither can be read back. Prefer reading;
edit only while trying things out.

### Start over

Stop the server, then `rm seatlane.db` (and `seatlane.db-journal` if there
is one). The next start creates a fresh database with the demo events; all
accounts, holds and bookings are gone (run `make seed` again for the dev
accounts).

### Limits and CSRF, in a nutshell

- **Sign-in rate limit**, per 15-minute window: 5 failed sign-ins per IP +
  email, and 20 failed sign-ins per IP across all emails. Past either, sign-in
  answers F18 (429, "too many sign-in attempts") until the window ends; a
  successful sign-in clears that email's count. While testing, wait, or `sqlite3 seatlane.db "DELETE FROM sign_in_attempts;"`.
- **CSRF**: every `POST` under `/api/` must send the `seatlane_csrf` cookie's
  value back in an `X-CSRF-Token` header. The web app does this for you;
  with curl, keep a cookie jar (`-c`/`-b`) and
  get the cookie first: any page sets it, and so does an F19 answer. A missing or stale token
  is F19 (403); the web app fetches a fresh one and retries once. Set
  `SEATLANE_CSRF_KEY` (64 hex characters, e.g. `openssl rand -hex 32`) to
  keep tokens valid across restarts; without it each start picks a random
  key.

### Troubleshooting

- **The server exits at startup with `seatlane.db was created by an older
  version; delete it (rm seatlane.db) and restart`.** The file was made by an
  earlier Seatlane whose tables differ (there are no migrations). Do what it
  says: `rm seatlane.db` and start again. The server checks this at startup
  (`internal/dbopen`: the schema version in `PRAGMA user_version` and every
  table's columns against `schema.sql`), and the line after the message
  names the difference, e.g. `seats.held_by is TEXT, want INTEGER`.
- **Every request fails with `internal error: sql: Scan error on column index
  6, name "held_by": converting driver.Value type string ("") to a int64:
  invalid syntax`.** The same cause: a `seatlane.db` from before phase 1
  (anonymous text sessions in `held_by`), run by a build without the startup
  check. `rm seatlane.db` and restart.
- **Sign-up says the password must be 10 to 72 bytes (F15)**: count bytes,
  not characters; each accented letter or emoji is 2 to 4 bytes.
- **"Email is already registered" (F16)**: sign in instead, or pick another
  email (or reset the database).
- **"Too many sign-in attempts" (F18)**: see the rate limit above.
- **403 `forbidden` when holding a seat**: you are signed in as an organizer
  or admin; only customers hold and book.
- **403 F19 from curl**: send the CSRF header (above).
- **`no web app at web/dist; serving the API only`**: run `make build` (or
  use `make dev` and port 5173).

## API

| Route | Handler | Who | Failure IDs |
|---|---|---|---|
| `GET /api/events` | list_events | everyone | F11, F12 |
| `GET /api/events/{id}/seats` | show_seat_map | everyone | F10, F11, F12 |
| `GET /api/events/{id}/holds` | list_my_holds | customer | F11, F12 |
| `POST /api/holds` `{"seat_id"}` | hold_seat | customer | F1, F6, F7 |
| `POST /api/holds/confirm` `{"seat_id"}` | confirm_hold | customer | F2, F3, F4, F5, F6, F7 |
| `POST /api/holds/confirm-all` `{"seat_ids"}` | confirm_holds | customer | F2, F13 |
| `POST /api/holds/release` `{"seat_id"}` | release_hold | customer | F5, F6, F7, F9 |
| `GET /api/me` | me | customer, organizer, admin | |
| `POST /api/sign-up` `{"email","password"}` | internal/auth | everyone | F14, F15, F16 |
| `POST /api/sign-in` `{"email","password"}` | internal/auth | everyone | F17, F18 |
| `POST /api/sign-out` | internal/auth | everyone | |

Any route that is not for everyone can also answer 401 `unauthorized` or 403
`forbidden`, and every `POST` can answer F19 (CSRF).

## Layout

```
cmd/server/        main (dbopen.Open, seed, -seed-dev-accounts, -dev-clock, CSRF) and routes.go (AppRoles, one httpx.Bind per slice, httpx.Identify)
features/<slice>/  intent.md, action.go, queries/, db/, checks/, <slice>.en
internal/domain/   value objects and pure seat rules (rendered into the English)
internal/auth/     accounts, bcrypt, sessions, the identity hook, CSRF, sign-in rate limit (app code + intent.md)
internal/dbopen/   opens seatlane.db, refusing one an older version created (PRAGMA user_version + columns)
internal/seed/     demo events and the dev-only accounts
internal/web/      serves web/dist with the SPA fallback (and the CSRF cookie)
web/               the React app
docs/              failures.md, screenshots/
```

## License

MIT. See [LICENSE](LICENSE).

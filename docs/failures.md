# Failure catalog

Every failure case in Seatlane has one app-wide F-ID. A slice declares only
the F-IDs it can raise, with the same status and message everywhere, so the
web app maps `error.id` to friendly copy without ever reading `error.message`.

| F-ID | Status | Message | Raised by |
|---|---|---|---|
| F1 | 409 | seat is already held | hold_seat |
| F2 | 410 | hold has expired | confirm_hold, confirm_holds |
| F3 | 409 | there is no hold to confirm | confirm_hold |
| F4 | 403 | seat is held by someone else | confirm_hold |
| F5 | 409 | seat is already confirmed | confirm_hold, release_hold |
| F6 | 409 | seat is already sold | hold_seat, confirm_hold, release_hold |
| F7 | 404 | seat does not exist | hold_seat, confirm_hold, release_hold |
| ~~F8~~ | | retired in phase 1 ("session is required"); replaced by the runtime's 401 `unauthorized` | |
| F9 | 409 | you have no hold on this seat | release_hold |
| F10 | 404 | event does not exist | show_seat_map |
| F11 | 400 | page limit is out of range | list_events, show_seat_map, list_my_holds |
| F12 | 400 | page cursor must be greater than zero | list_events, show_seat_map, list_my_holds |
| F13 | 409 | a seat in the list is not held by you | confirm_holds |
| F14 | 400 | email is not valid | sign_up |
| F15 | 400 | password must be 10 to 72 bytes | sign_up |
| F16 | 409 | email is already registered | sign_up |
| F17 | 401 | email or password is wrong | sign_in (unknown email and wrong password alike) |
| F18 | 429 | too many sign-in attempts, try again later | sign_in (with `Retry-After` in seconds) |
| F19 | 403 | missing or invalid CSRF token | any POST/PUT/PATCH/DELETE under `/api/` |

F-IDs are never reused: F8 stays retired.

## Access answers (bridge-en runtime, no F-ID)

| `error.id` | Status | Message | When |
|---|---|---|---|
| `unauthorized` | 401 | sign-in required | the action is not Public and nobody is signed in |
| `forbidden` | 403 | not allowed for this role | signed in, but the action's `Roles` does not list the role |
| `bad_request` | 400 | (varies) | malformed input, or the request sent a server-set value (`user`, `role`, `now`) |

Both access answers come from `httpx.Bind` before the action runs, so nothing
is read or written. Roles per action: `list_events` and `show_seat_map` are
Public; `hold_seat`, `confirm_hold`, `confirm_holds`, `release_hold` and
`list_my_holds` are for `customer`; `me` is for every role. Sign-up, sign-in
and sign-out are open to everyone.

## Holds

The hold time is 600 seconds (10 minutes). A hold taken at `held_at` has
`expires_at = held_at + 600`; it is active while `expires_at` is later than
the current time and expired from `expires_at` on (a hold taken exactly 600
seconds ago has expired; one taken 599 seconds ago has not).

`held_by` and `sold_to` store the user id (`users.id`), with `0` meaning
nobody. Each action gets the caller only as `server:"user"` (and `me` also
gets `server:"role"`), which `httpx.Identify` fills from the `seatlane_auth`
cookie through the app's identity hook (`internal/auth.Identify`). A request
that sends `user`, `role` or `now` itself, in the body or the query string, is
answered 400 `bad_request`.

`confirm_holds` is all or none: on F2 or F13 no seat in the list is sold
and every change is rolled back.

## Sign-in and CSRF

- F17 is the same answer, status and timing for an unknown email and a wrong
  password: an unknown email is still checked against a dummy bcrypt hash of
  the same cost.
- F18: at most 5 failed sign-ins per client IP + email, and at most 20 per
  client IP across all emails, each in a 15-minute window that starts at its
  first failure. The next attempt over either cap is refused before the
  password is checked, even if it is right. A window starts again 900 seconds
  after its first failure (899 s: still refused; 900 s: allowed). A
  successful sign-in clears its IP + email count and gives back its own
  attempt in the IP count. There is no per-email cap across IPs.
- F19: a state-changing `/api/` request needs the header `X-CSRF-Token` equal
  to the `seatlane_csrf` cookie, a valid HMAC token bound to the current
  sign-in session, and no cross-site `Origin`/`Sec-Fetch-Site`. The F19
  answer carries a fresh cookie, so the web app retries once.

Error answers are `{"error": {"id": "F2", "message": "hold has expired"}}`
(the bridge-en runtime's `httpx.ErrorBody`): `error.id` is the stable field.

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
| F8 | 401 | session is required | hold_seat, confirm_hold, confirm_holds, release_hold, show_seat_map, list_my_holds |
| F9 | 409 | you have no hold on this seat | release_hold |
| F10 | 404 | event does not exist | show_seat_map |
| F11 | 400 | page limit is out of range | list_events, show_seat_map, list_my_holds |
| F12 | 400 | page cursor must be greater than zero | list_events, show_seat_map, list_my_holds |
| F13 | 409 | a seat in the list is not held by this session | confirm_holds |

The hold time is 600 seconds (10 minutes). A hold taken at `held_at` has
`expires_at = held_at + 600`; it is active while `expires_at` is later than
the current time and expired from `expires_at` on (a hold taken exactly 600
seconds ago has expired; one taken 599 seconds ago has not).

The session is the cookie `bridge_session`, which the bridge-en runtime reads
into each action's `session` input (`server:"session"`, a string); without a
valid cookie the session is the empty text and the action answers F8. A
session is 128 random bits from crypto/rand, written as 26 base32
characters; `held_by` and `sold_to` store it as TEXT, with `''` meaning
nobody. The server issues the cookie on every response that serves the web
app's HTML (`internal/session.Page`) and, as a fallback, on any API call
without a valid one (`internal/session.Issue`): HttpOnly, SameSite=Lax,
Path=/, and Secure over TLS. A request that sends `session` itself, in the
body or the query string, is answered 400 `bad_request`.

`confirm_holds` is all or none: on F2 or F13 no seat in the list is sold
and every change is rolled back.

Error answers are `{"error": {"id": "F2", "message": "hold has expired"}}`
(the bridge-en runtime's `httpx.ErrorBody`): `error.id` is the stable field.

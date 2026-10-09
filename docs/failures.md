# Failure catalog

Every failure case in Seatlane has one app-wide F-ID. A slice declares only
the F-IDs it can raise, with the same status and message everywhere, so the
web app maps `error.id` to friendly copy without ever reading `error.message`.

| F-ID | Status | Message | Raised by |
|---|---|---|---|
| F1 | 409 | seat is already held | hold_seat |
| F2 | 410 | hold has expired | confirm_hold |
| F3 | 409 | there is no hold to confirm | confirm_hold |
| F4 | 403 | seat is held by someone else | confirm_hold |
| F5 | 409 | seat is already confirmed | confirm_hold, release_hold |
| F6 | 409 | seat is already sold | hold_seat, confirm_hold, release_hold |
| F7 | 404 | seat does not exist | hold_seat, confirm_hold, release_hold |
| F8 | 401 | session is required | hold_seat, confirm_hold, release_hold, show_seat_map, list_my_holds |
| F9 | 409 | you have no hold on this seat | release_hold |
| F10 | 404 | event does not exist | show_seat_map |
| F11 | 400 | page limit is out of range | list_events, show_seat_map, list_my_holds |
| F12 | 400 | page cursor must be greater than zero | list_events, show_seat_map, list_my_holds |

The hold time is 600 seconds (10 minutes). A hold taken at `held_at` has
`expires_at = held_at + 600`; it is active while `expires_at` is later than
the current time and expired from `expires_at` on (a hold taken exactly 600
seconds ago has expired; one taken 599 seconds ago has not).

The session is the cookie `bridge_session`, which the bridge-en runtime reads
into each action's `session` input (`server:"session"`); without a valid
cookie the session is 0 and the action answers F8. The server issues the
cookie (`internal/session.Issue`); a request that sends `session` itself, in
the body or the query string, is answered 400 `bad_request`.

Error answers are `{"error": {"id": "F2", "message": "hold has expired"}}`
(the bridge-en runtime's `httpx.ErrorBody`): `error.id` is the stable field.

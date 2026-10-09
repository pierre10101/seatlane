# Intent: Hold seat

## Why
A visitor picks a seat and holds it for 10 minutes while deciding. Two people
must never hold the same seat at the same moment, and a hold that is not
confirmed in time frees the seat for the next person.

## Who
Any visitor. The holder is the anonymous session from the `seatlane_sid`
cookie; the server puts it into the request as `session` (the caller never
sends it).

## Inputs
- `seat_id` — the seat to hold.
- `session` — the visitor's anonymous session id (set by the server from the cookie).
- `now` — the current time, set by the server (whole seconds since 1970).

## Outputs
- `seat_id`, `held_at`, `expires_at` and the server's `now`: the hold that
  was taken. `expires_at` is `now` + 600; the web app counts down from
  `expires_at - now` as the server reported them, never from its own clock.

## Rule
One conditional UPDATE claims the seat only if, at that moment, it is not
sold and either nobody holds it or its hold has expired (`expires_at` is no
later than now). It must change exactly one row; there is no read before the
write. A hold whose `expires_at` equals now has expired and no longer blocks
the seat; one that expires one second later still does.

## Failure cases
- **F1** — the seat is already held: someone (another visitor, or you) holds it
  and the hold has not expired. Nothing changes.
- **F6** — the seat is already sold. Nothing changes.
- **F7** — the seat does not exist. Nothing changes.
- **F8** — the session is missing (zero or negative). Nothing changes.

## Out of scope
Confirming (confirm_hold) and releasing (release_hold) a hold; payment.

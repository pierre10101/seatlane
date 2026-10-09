# Intent: Hold seat

## Why
A customer picks a seat and holds it for 10 minutes while deciding. Two people
must never hold the same seat at the same moment, and a hold that is not
confirmed in time frees the seat for the next person.

## Who
A signed-in customer (`httpx.Roles("customer")`). The bridge-en runtime
answers HTTP 401 `unauthorized` to a caller who is not signed in and HTTP 403
`forbidden` to a signed-in organizer or admin, before the action runs and
without writing anything. The customer is the `user` input (their account
id), which the runtime sets from Seatlane's sign-in session (the
`seatlane_auth` cookie, looked up by `internal/auth`); the caller never sends
it, and a request that does is answered with HTTP 400.

## Inputs
- `seat_id` — the seat to hold.
- `user` — the signed-in customer's account id (set by the server).
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
- F1: the seat is already held: someone (another customer, or you) holds it
  and the hold has not expired. Nothing changes.
- F6: the seat is already sold. Nothing changes.
- F7: the seat does not exist. Nothing changes.

## Out of scope
Confirming (confirm_hold) and releasing (release_hold) a hold; payment.

# Intent: Confirm hold

## Why
A customer who holds a seat confirms it before the hold expires; the seat is
then sold to them and nobody else can hold it.

## Who
A signed-in customer (`httpx.Roles("customer")`). The bridge-en runtime
answers HTTP 401 `unauthorized` to a caller who is not signed in and HTTP 403
`forbidden` to a signed-in organizer or admin, before the action runs and
without writing anything. The customer is the `user` input (their account
id), which the runtime sets from Seatlane's sign-in session (the
`seatlane_auth` cookie, looked up by `internal/auth`); the caller never sends
it, and a request that does is answered with HTTP 400.

## Inputs
- `seat_id` — the seat to confirm.
- `user` — the signed-in customer's account id (set by the server).
- `now` — the current time, set by the server.

## Outputs
- `seat_id`, `sold_at` (= now) and the server's `now`.

## Rule
One conditional UPDATE sells the seat only if, at that moment, it is not sold,
it is held by this customer and the hold has not expired (`expires_at` is later
than now). It must change exactly one row. When it changes nothing, reads made
after it (never before) explain why. A hold whose `expires_at` equals now has
expired: confirming at that second is F2.

## Failure cases
- F2: the hold expired: the seat is still held by this customer but its
  `expires_at` is now or earlier (the hold was taken 600 seconds or more ago).
  Nothing changes.
- F3: confirm after release: nobody holds the seat (it was released, or
  never held). Nothing changes.
- F4: confirm another person's hold: the seat is held by another customer.
  Nothing changes.
- F5: double confirm: the seat is already sold to this customer. Nothing
  changes.
- F6: the seat is already sold to someone else. Nothing changes.
- F7: the seat does not exist. Nothing changes.

## Out of scope
Payment, refunds.

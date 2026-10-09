# Intent: Release hold

## Why
A customer changes their mind and gives a held seat back at once, instead of
letting the hold run out.

## Who
A signed-in customer (`httpx.Roles("customer")`). The bridge-en runtime
answers HTTP 401 `unauthorized` to a caller who is not signed in and HTTP 403
`forbidden` to a signed-in organizer or admin, before the action runs and
without writing anything. The customer is the `user` input (their account
id), which the runtime sets from Seatlane's sign-in session (the
`seatlane_auth` cookie, looked up by `internal/auth`); the caller never sends
it, and a request that does is answered with HTTP 400.

## Inputs
- `seat_id` — the seat to release.
- `user` — the signed-in customer's account id (set by the server).
- `now` — the current time, set by the server.

## Outputs
- `seat_id`, `released_at` (= now) and the server's `now`.

## Rule
One conditional UPDATE clears the hold only if, at that moment, the seat is
held by this customer and not sold. An expired hold of this customer that
nobody else has taken yet can still be released (it frees the seat either
way). It must change exactly one row.

## Failure cases
- F5: the seat is already confirmed (sold to this customer): a sale is not
  released. Nothing changes.
- F6: the seat is sold to someone else. Nothing changes.
- F7: the seat does not exist. Nothing changes.
- F9: this customer has no hold on the seat (nobody holds it, or someone
  else does). Nothing changes.

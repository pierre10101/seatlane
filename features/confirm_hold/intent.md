# Intent: Confirm hold

## Why
A visitor who holds a seat confirms it before the hold expires; the seat is
then sold to them and nobody else can hold it.

## Who
The visitor whose anonymous session holds the seat. The session is the
`session` input, which the bridge-en runtime sets from the session cookie
`bridge_session` (the empty text without a valid cookie); the caller never sends it.

## Inputs
- `seat_id` — the seat to confirm.
- `session` — the visitor's session id (set by the server from the cookie).
- `now` — the current time, set by the server.

## Outputs
- `seat_id`, `sold_at` (= now) and the server's `now`.

## Rule
One conditional UPDATE sells the seat only if, at that moment, it is not sold,
it is held by this session and the hold has not expired (`expires_at` is later
than now). It must change exactly one row. When it changes nothing, reads made
after it (never before) explain why. A hold whose `expires_at` equals now has
expired: confirming at that second is F2.

## Failure cases
- F2: the hold expired: the seat is still held by this session but its
  `expires_at` is now or earlier (the hold was taken 600 seconds or more ago).
  Nothing changes.
- F3: confirm after release: nobody holds the seat (it was released, or
  never held). Nothing changes.
- F4: confirm another person's hold: the seat is held by another session.
  Nothing changes.
- F5: double confirm: the seat is already sold to this session. Nothing
  changes.
- F6: the seat is already sold to someone else. Nothing changes.
- F7: the seat does not exist. Nothing changes.
- F8: the session is missing (the empty text: no valid cookie). Nothing
  changes.

## Out of scope
Payment, refunds.

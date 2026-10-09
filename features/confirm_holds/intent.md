# Intent: Confirm holds

## Why
A visitor who holds several seats of an event confirms them together from the
review screen. Every seat in the list is confirmed, or none are: a group where
one seat can no longer be sold must never end up half booked.

## Who
The visitor whose anonymous session holds the seats. The session is the
`session` input, which the bridge-en runtime sets from the session cookie
`bridge_session` (the empty text without a valid cookie); the caller never sends it.

## Inputs
- `seat_ids` — the seats shown on the review screen: 1 to 20 seat ids, each at
  most once. An empty list, more than 20 ids, a repeated id, a null or an id
  that is not a whole number is refused with HTTP 400 before anything runs.
- `session` — the visitor's session id (set by the server from the cookie).
- `now` — the current time, set by the server.

## Outputs
- `confirmed` — how many seats were sold: always the number of ids sent.
- `sold_at` (= now) and the server's `now`.

## Rule
One conditional UPDATE sells every listed seat only if, at that moment, it is
not sold, it is held by this session and its hold has not expired
(`expires_at` is later than now); `id IN (sqlc.slice(seat_ids))` is its last
parameter. It must change exactly one row per listed seat; otherwise the
transaction rolls back and no seat in the list is sold. Seats this session
holds that are not in the list are never touched. A read made after the
UPDATE (never before) explains an expired hold: it counts the listed seats
this session still holds, unsold, whose `expires_at` is no later than now.
A hold whose `expires_at` equals now has expired; one that expires one second
after now has not.

## Failure cases
- F2: at least one listed seat is still held by this session but its hold
  has expired (`expires_at` is now or earlier): no seat is sold, every change
  is rolled back.
- F8: the session is missing (the empty text: no valid cookie): nothing
  is written.
- F13: at least one listed seat is not held by this session (held by
  someone else, including a seat whose hold by this session expired and was
  then taken by someone else; released or never held; already sold; or no
  such seat): no seat is sold, every change is rolled back.

## Out of scope
Taking and releasing holds; confirming one seat (confirm_hold); payment.

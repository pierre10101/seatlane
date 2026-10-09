# Intent: Release hold

## Why
A visitor changes their mind and gives a held seat back at once, instead of
letting the hold run out.

## Who
The visitor whose session holds the seat (cookie `seatlane_sid`, put into the
request as `session` by the server).

## Inputs
- `seat_id` — the seat to release.
- `session` — the visitor's session id (set by the server).
- `now` — the current time, set by the server.

## Outputs
- `seat_id`, `released_at` (= now) and the server's `now`.

## Rule
One conditional UPDATE clears the hold only if, at that moment, the seat is
held by this session and not sold. An expired hold of this session that
nobody else has taken yet can still be released (it frees the seat either
way). It must change exactly one row.

## Failure cases
- **F5** — the seat is already confirmed (sold to this session): a sale is not released.
- **F6** — the seat is sold to someone else.
- **F7** — the seat does not exist.
- **F8** — the session is missing (zero or negative).
- **F9** — this session has no hold on the seat (nobody holds it, or someone
  else does).

Every failure changes nothing.

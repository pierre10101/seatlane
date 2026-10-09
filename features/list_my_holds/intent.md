# Intent: List my holds

## Why
The seat map shows which seats the visitor holds (`held_by_me`) but never
when any hold ends: other visitors must not learn when someone else's hold
runs out. The visitor still needs the countdown for their own holds, so this
read lists only the seats their session holds, each with its `expires_at`.

## Inputs
- `event_id` (path) — the event.
- `after`, `limit` (query) — keyset paging over seat ids, highest first
  (`limit` 1..100, default 20; `after` defaults to the start).
- `session` — the visitor's session id, set by the server from the session
  cookie `bridge_session` (the empty text without a valid cookie); the caller never sends it.
- `now` — the current time, set by the server.

## Outputs
- `holds`: one page of the seats of this event that this session holds and
  that are not sold, each with `seat_id`, `held_at`, `expires_at` and
  `active` (true while `expires_at` is later than now). An expired hold of
  this session is listed with `active` false until someone else holds the seat.
- `next_after`: the cursor of the next page, 0 on the last page.
- `now`: the server's current time, for countdowns.

An event that does not exist simply has no holds: the list is empty.

## Failure cases
- F8: the session is missing (the empty text: no valid cookie).
- F11: `limit` is not between 1 and 100.
- F12: `after` is zero or negative.

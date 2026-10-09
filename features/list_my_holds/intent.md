# Intent: List my holds

## Why
The seat map shows which seats the customer holds (`held_by_me`) but never
when any hold ends: other customers must not learn when someone else's hold
runs out. The customer still needs the countdown for their own holds, so this
read lists only the seats they hold, each with its `expires_at`.

## Who
A signed-in customer (`httpx.Roles("customer")`). The bridge-en runtime
answers HTTP 401 `unauthorized` to a caller who is not signed in and HTTP 403
`forbidden` to a signed-in organizer or admin, before the action runs. The
customer is the `user` input (their account id), set by the server from
Seatlane's sign-in session; the caller never sends it.

## Inputs
- `event_id` (path) — the event.
- `after`, `limit` (query) — keyset paging over seat ids, highest first
  (`limit` 1..100, default 20; `after` defaults to the start).
- `user` — the signed-in customer's account id (set by the server).
- `now` — the current time, set by the server.

## Outputs
- `holds`: one page of the seats of this event that this customer holds and
  that are not sold, each with `seat_id`, `held_at`, `expires_at` and
  `active` (true while `expires_at` is later than now). An expired hold of
  this customer is listed with `active` false until someone else holds the seat.
- `next_after`: the cursor of the next page, 0 on the last page.
- `now`: the server's current time, for countdowns.

An event that does not exist simply has no holds: the list is empty.

## Failure cases
- F11: `limit` is not between 1 and 100.
- F12: `after` is zero or negative.

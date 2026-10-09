# Intent: Show seat map

## Why
The seat map shows every seat of an event and its state for the viewer, so
the web app only draws what the server says: it never decides by itself
whether a seat is free or a hold has expired.

## Who
Anyone, signed in or not (`httpx.Public`): browsing seats needs no account.
The viewer is the `user` input, which the bridge-en runtime sets from
Seatlane's sign-in session (the `seatlane_auth` cookie, looked up by
`internal/auth`), or 0 when the caller is not signed in; the caller never
sends it. A viewer who is not signed in sees every seat as `available`,
`held_by_other` or `sold_to_other`, never as theirs.

## Inputs
- `event_id` (path) — the event.
- `after`, `limit` (query) — keyset paging over seat ids, highest first
  (`limit` 1..100, default 20; `after` defaults to the start).
- `user` — the signed-in viewer's account id, or 0 (set by the server).
- `now` — the current time, set by the server.

## Outputs
- `event`: the event card.
- `seats`: one page of seats, each with section, row, number, price and the
  flags `available`, `held_by_me`, `held_by_other`, `sold_to_me`,
  `sold_to_other` (exactly one is true).

The seat map never sends a hold's `expires_at`: not for other visitors'
holds (that would tell everyone when someone else's hold runs out), and not
for the viewer's own either, because the slice grammar cannot send a value
only for some rows. The viewer's own hold times come from List my holds
(`GET /api/events/{id}/holds`), which only reads seats the signed-in customer holds.
- `next_after`: the cursor of the next page, 0 on the last page.
- `now`: the server's current time, for countdowns.

A hold counts as active while its `expires_at` is later than now.

## Failure cases
- F10: the event does not exist.
- F11: `limit` is not between 1 and 100.
- F12: `after` is zero or negative.

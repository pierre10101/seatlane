# Intent: List events

## Why
The home page lists the events on sale, newest first, as cards.

## Who
Anyone, signed in or not (`httpx.Public`).

## Inputs
- `after`, `limit` (query) — keyset paging over event ids, highest first
  (`limit` 1..100, default 20).

## Outputs
- `events`: one page of event cards (name, venue, city, start time, tagline,
  lowest and highest seat price: `from_price` and `to_price`).
- `next_after`: the cursor of the next page, 0 on the last page.

## Failure cases
- F11: `limit` is not between 1 and 100 (both included; 0 and 101 are
  refused, 1 and 100 are allowed): nothing is read.
- F12: `after` is sent and is zero or negative (leaving it out starts at the
  newest event): nothing is read.

# Intent: List events

## Why
The home page lists the events on sale, newest first, as cards.

## Inputs
- `after`, `limit` (query) — keyset paging over event ids, highest first
  (`limit` 1..100, default 20).

## Outputs
- `events`: one page of event cards (name, venue, city, start time, tagline,
  lowest price).
- `next_after`: the cursor of the next page, 0 on the last page.

## Failure cases
- **F11** — `limit` is not between 1 and 100.
- **F12** — `after` is zero or negative.

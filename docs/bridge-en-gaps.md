# bridge-en gaps (for the v0.2 list)

Found while building "Confirm all" on bridge-en v0.1.2 (2026-10-09). Seatlane
does not modify bridge-en and uses no waivers, so these block the feature.

## G1: an all-or-nothing confirm of an explicit list of seats

Wanted (reviewer): `POST` with `seat_ids` (the seats on the review screen,
1 to 20, no duplicates); one conditional UPDATE sells only those seats where
`held_by` = session, `sold_to` = '' and `expires_at` > now; then stop, rolling
everything back, unless the number of rows changed is the number of seats
requested. The English must say "only the seats in the request's `seat_ids`"
and "if the number changed is not the number of seats requested".

v0.1.2 refuses every piece of it (exact messages from `bridge-en -check`):

1. **List input (D4).** `SeatIDs []int64 \`json:"seat_ids"\`` -
   `refused: field type []int64 is not in the allowed pattern list (D4 input). List fields are only on Output (D5); Input stays scalar`
2. **IN / sqlc.slice in a claim (Q6).** `WHERE id IN (sqlc.slice(seat_ids)) AND ...` -
   `refused: IN is not in the allowed pattern list (query ConfirmSeats). Expected a comparison (= <> < <= > >=) after id`
3. **Changed-row count against a request value (S10).** `if confirmed != in.Count` -
   `refused: comparison confirmed != in.Count on a claim's changed-row count is not in the allowed pattern list (S10 claim check). Compare a Q6 result only as != 1 (the check), == 1 or == 0`
4. **Length of a list (E5).** `len(in.SeatIDs)` -
   `refused: call to len is not in the allowed pattern list (expression)`
5. **List validation.** No rule renders "between 1 and 20 items" or "no
   duplicates" for an input list (page.IsPageLimit is only for scalars).

Needed in v0.2: a bounded scalar-list input (e.g. `[]int64` with a declared
max, duplicates refused by httpx.Bind, English "a list of 1 to 20 distinct
whole numbers"); a Q6 condition `<col> IN (sqlc.slice(<list>))` rendered as
"only the seats whose `id` is in the request's `seat_ids`"; and an S10 form
`if <n> != len(in.<List>)` rendered as "if the number of seats changed in
step k is not the number of seats requested".

Workarounds deliberately not used: N single-seat claims, a fixed-arity
`seat_1..seat_8` input (Input has at most 10 fields, two are server-set),
or an OR group of fixed slots.

## G2: S10 accepts `!= 1` buried in a compound guard

S10 counts a claim as checked when `<n> != 1` appears anywhere inside a guard
condition, so `if n == 0 && n != 1 { return Output{}, F13 }` passes and
renders "If no seat was changed in step 2 and not exactly one seat was
changed in step 2, stop with F13". The English is true but redundant, and it
lets a multi-row claim pass the "exactly one row" check without meaning it.
Either S10 should require the bare `if <n> != 1` guard, or (better, with G1)
offer a real multi-row check. Also, a plain `if n == 0` guard is not accepted
as a claim check, so a slice cannot write "if no seat was changed" on its own.

## G3: S10 assumes one-row claims

Any Q6 claim must be followed by "stop unless exactly one row changed". A
multi-row conditional UPDATE (every seat of an event held by this session)
can only pass by G2's loophole. An all-or-nothing multi-row transition needs
its own rule (G1 item 3).

# bridge-en gaps

G1-G3 were found on v0.1.2; G4-G11 on v0.3.0 (phase 1: accounts, roles,
CSRF and sign-in rate limits). Seatlane does not modify bridge-en and uses
no waivers.

**Status (2026-10-09): G1-G3 are closed by bridge-en v0.1.3** (D10 list
inputs, Q7 `id IN (sqlc.slice(...))` claims, the S11 multi-row check and the
strict S10 check). Seatlane uses them in `features/confirm_holds`. The notes
below are kept as the record of what v0.1.2 refused.

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

## Phase 1 on v0.3.0 (2026-10-09)

v0.3.0's T3/A1-A3 (`server:"user"`, `server:"role"`, `httpx.Roles`,
`httpx.AppRoles`, `httpx.Identify`) cover authorization: every seatlane
slice declares Roles and takes the caller only from `server:"user"`. The
*authentication* side (sign-up, sign-in, sign-out, the CSRF token) cannot be
written as slices, so it lives in `internal/auth` as app code. Each of those
flows still has an `intent.md` (F14-F19) written first, and
`internal/auth/intent_test.go` cross-checks the F-IDs in the intent files, the
code and the checks the way `bridge-en -check` does for slices. None of these
flows has a `.en` file. The messages below are exact output from
`bridge-en -check` on a probe slice.

### G4: no password hashing in a slice or in the domain

`refused: import "golang.org/x/crypto/bcrypt" is not in the allowed pattern
list (D2 imports). Allowed: context, net/http,
github.com/pierre10101/go-ai-bridge/runtime/{assert,failure,page,httpx},
<module>/internal/domain, <module>/features/<slice>/db`. Domain files (M2/M3)
may import only fmt, assert and shape, so a domain helper cannot call bcrypt
either. Wanted: a runtime `password` package (hash with bcrypt or argon2id,
plus a constant-time verify that also burns the same time for an unknown
email), rendered as "the password is stored only as a bcrypt hash" and
"compare the password with the stored hash".

### G5: no client-IP server-set input

`refused: server field IP tagged server:"ip" is not in the allowed pattern
list (T2 session is passed in)`. A per-IP+email sign-in limit needs the
client address as a server-set value, like `now`. Wanted: `server:"client_ip"`
set from `RemoteAddr` (with an explicit, opt-in trusted-proxy rule),
rendered as "the caller's network address (set by the server)".

### G6: no cookie (or any header) on an answer

Output is D5 JSON fields only (`refused: field Cookie without a json tag is
not in the allowed pattern list (D5 output)`), and the runtime only *reads*
cookies (`httpx.SessionCookie`, the `httpx.Identity` hook). Starting or
ending a session (Set-Cookie with HttpOnly/SameSite/Secure) and issuing a CSRF
token cannot be part of an action. Wanted: a declared "starts the signed-in
session for user X" / "ends the signed-in session" output that the runtime
turns into the cookie, so the English can say it.

### G7: no upsert (Q3 refuses ON CONFLICT)

`refused: ON CONFLICT (upsert) is not in the allowed pattern list (query
...). Expected RETURNING <col>, ...`. Sign-up wants
`INSERT ... ON CONFLICT (email) DO NOTHING RETURNING id` ("register the email
only if it is not registered yet; F16 if it is"), the race-free counterpart of
a Q6 claim. The attempts counter wants `ON CONFLICT (key) DO UPDATE`. W1
forbids the check-then-insert alternative, rightly, so there is no legal shape.
Wanted: Q3 with `ON CONFLICT (<unique col>) DO NOTHING` plus an S10-style check on the returned row:
"if the email was already registered, stop with F16".

### G8: no DELETE

`refused: DELETE statement is not in the allowed pattern list (query ...).
Expected SELECT, INSERT or UPDATE`. Sign-out (delete the session row) and the
expired-session sweep need it. Wanted: a Q-rule for a keyed DELETE (`WHERE
<key> = ...`, and a bounded `WHERE expires_at <= now` sweep) with a
changed-row count, like Q6.

### G9: no 429 / Retry-After

A failure is `failure.New(id, status, message)`. There is no way to declare a
rate limit, attach `Retry-After`, or render "after 5 failed attempts in 15
minutes from the same address for the same email, refuse with F18 until the
window ends". The pure rule (`internal/auth.Reserve(prev, now)`) is tested,
but it is not in any English. Wanted: a declared limiter (key from server-set
values, window, max) in the runtime, rendered in the contract.

### G10: unknown query parameters are accepted on GET

POST bodies refuse unknown fields (`DisallowUnknownFields`), but a GET slice
ignores query parameters it does not declare: `GET
/api/events/1/seats?role=admin` is a 200. It is harmless here, because a
declared server-set name (`user`, `role`, `now`) is a 400, but it is
inconsistent and hides client typos (`?limt=5`). Wanted: QueryInputRule
refuses undeclared parameters, or the English says it ignores them.

### G11: a Public slice can only see "signed out" as user 0

`show_seat_map` is Public but marks a viewer's own seats, so it takes `User
int64 server:"user"`, and a signed-out viewer is 0 (`SignedOutZero`). That
works, but the domain has to repeat the rule "0 owns nothing" (`heldBy != 0
&& heldBy == viewer`), and the English says "0" instead of "a signed-out
viewer". Wanted: the English names a signed-out viewer, or the runtime offers
a typed "signed in or not" value.

### Not attempted: organizer ownership

Organizers exist as a role (`AppRoles` has customer, organizer and admin), and
no slice grants them anything yet. Rules like "an organizer may change only
their own events" wait for bridge-en rule A4, as planned.

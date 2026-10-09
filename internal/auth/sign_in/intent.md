# Intent: Sign in

`POST /api/sign-in` `{"email", "password"}`. App code in `internal/auth`
(see docs/bridge-en-gaps.md G4), with one `TestF<n>_...` check per F-ID
against real SQLite (`internal/auth/intent_test.go` enforces it).

## Why
A customer, organizer or admin proves who they are with their email and
password and gets a sign-in session, so the server knows their account and
role on every later request.

## Who
Anyone. Needs the CSRF token like every state-changing request under
`/api/` (F19).

## Inputs
- `email` — trimmed and lower-cased, as at sign-up.
- `password` — exactly as sent.
- the client's IP address — the host part of the connection's remote
  address (`X-Forwarded-For` is not trusted).
- the current time `now` — passed in from the server clock (`httpx.Now`);
  the rate-limit rule itself is a pure function of the stored count and `now`.
A body that is not one JSON object with exactly these two text fields is
HTTP 400 `bad_request` (and is not counted as an attempt).

## Outputs
HTTP 200 with `{"user_id", "email", "role"}`. The answer sets a new sign-in
cookie `seatlane_auth` (a fresh token on every sign-in, so a token planted
before sign-in is never promoted) and a new CSRF cookie bound to it.

## Rule
Rate limit, two caps checked together, each with its own window of 900
seconds that starts at its key's first counted attempt:
- per client IP + email (key `<ip>|<email>`): at most 5 attempts that did
  not succeed;
- per client IP across all emails (key `ip:<ip>`): at most 20 attempts that
  did not succeed.
There is deliberately no cap per email across IPs (anyone could then lock an
account's owner out from anywhere).

Before the password is checked, one write transaction (it holds the database's
write lock) reads both counts. For each, if its window started 900 seconds or
more before now (`window_start + 900 <= now`), the count starts again from 0
with `window_start = now`. If either count is then at its cap (5, or 20), the
attempt is refused (F18) without checking the password, and neither count is
raised. Otherwise both counts go up by one and are saved before the password
is checked, so parallel guesses cannot pass either cap. A successful sign-in
deletes the IP + email count and gives back its own one in the IP count (if
that window has not started again), so only failures use up the IP's 20,
and one known password cannot reset the IP's count. Other IPs have their own
counts; the same IP with another email shares the IP count only.

The rules are pure functions of the stored counts and `now`
(`ReserveBoth`, `Refund` in `internal/auth/ratelimit.go`).

The password is checked with bcrypt against the stored hash. For an email
with no account, bcrypt still runs against a fixed dummy hash, so the answer
and its timing do not tell whether the email has an account.

## Failure cases
- F17: the email has no account, or the password does not match the
  account's hash. Both answer exactly the same: HTTP 401, id F17, "email or
  password is wrong". No session is created; the attempt counts toward the
  rate limit.
- F18: rate-limited: in the current window of a cap, which started less
  than 900 seconds ago, 5 attempts that did not succeed were made for this
  IP and email, or 20 for this IP across all emails (at `window_start + 899`
  still refused; at `window_start + 900` allowed again). HTTP 429 with a
  `Retry-After` header of the seconds until every cap that refused has
  ended its window. The password is not checked, no session is created, and
  no count is raised.

## Out of scope
Account lockout across IPs, CAPTCHA, multi-factor sign-in, "remember me".

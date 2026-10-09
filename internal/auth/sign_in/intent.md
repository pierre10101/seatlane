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
Rate limit, per client IP + email (key `<ip>|<email>`): at most 5 attempts
that did not succeed in a window of 900 seconds that starts at the first
such attempt. Before the password is checked, one write transaction reads
the key's count: if the window started 900 seconds or more before now
(`window_start + 900 <= now`) the count starts again from 0 with
`window_start = now`; if the count is then 5 or more, the attempt is refused
(F18) without checking the password; otherwise the count goes up by one and
is saved before the password is checked, so parallel guesses cannot pass the
limit. A successful sign-in deletes the key's count. Other IPs, and the same
IP with another email, have their own counts.

The password is checked with bcrypt against the stored hash. For an email
with no account, bcrypt still runs against a fixed dummy hash, so the answer
and its timing do not tell whether the email has an account.

## Failure cases
- F17: the email has no account, or the password does not match the
  account's hash. Both answer exactly the same: HTTP 401, id F17, "email or
  password is wrong". No session is created; the attempt counts toward the
  rate limit.
- F18: rate-limited: 5 attempts that did not succeed were made for this IP
  and email in the current window, which started less than 900 seconds ago
  (at `window_start + 899` still refused; at `window_start + 900` allowed
  again). HTTP 429 with a `Retry-After` header of the seconds left. The
  password is not checked, no session is created, and the count is not
  raised.

## Out of scope
Account lockout across IPs, CAPTCHA, multi-factor sign-in, "remember me".

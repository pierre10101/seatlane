# Intent: Sign up

`POST /api/sign-up` `{"email", "password"}`. App code in `internal/auth`
(password hashing and sessions live in Seatlane, not in bridge-en; see
docs/bridge-en-gaps.md G4), held to the same discipline as a slice: the
F-IDs below are exactly the failures `internal/auth` declares for sign-up,
and each has a `TestF<n>_...` check against real SQLite
(`internal/auth/intent_test.go` enforces both).

## Why
A visitor creates a customer account so their holds and bookings belong to
them, not to a browser.

## Who
Anyone (not signed in, or signed in: signing up signs the new account in
and replaces any current sign-in). Like every state-changing request under
`/api/`, it needs the CSRF token (F19, see `internal/auth/csrf/intent.md`).

## Inputs
- `email` — trimmed of spaces and lower-cased before anything else; that
  form is what is checked, stored and compared.
- `password` — kept exactly as sent (no trimming).
A body that is not one JSON object with exactly these two text fields is
HTTP 400 `bad_request`.

## Outputs
HTTP 201 with `{"user_id", "email", "role"}`; `role` is always `customer`
(organizers and admins are not created by sign-up). The answer sets a new
sign-in cookie `seatlane_auth` (HttpOnly, SameSite=Lax, Path=/, Secure over
TLS, 7 days) and a new CSRF cookie bound to it.

## Rule
The password is hashed with bcrypt (golang.org/x/crypto/bcrypt, cost 12)
before the account is written; the plain password is never stored or
logged. The account is inserted with one statement that writes only if no
account has that email (`INSERT ... ON CONFLICT (email) DO NOTHING`), so two
sign-ups with one email cannot both succeed. The new session's token is 256
random bits; only its SHA-256 is stored, with `expires_at` = now + 7 days.
The current time is passed in (the server clock `httpx.Now`).

## Failure cases
- F14: the email is not valid: after trimming and lower-casing it is empty,
  longer than 254 characters, has no "@" or more than one, has nothing
  before or after the "@", has no "." in the part after the "@", or contains
  a space or control character. Nothing is written.
- F15: the password is shorter than 10 bytes or longer than 72 bytes (the
  bcrypt limit; 10 and 72 are allowed). Nothing is written.
- F16: an account with this email already exists. Nothing is written and
  the existing account is unchanged.

## Out of scope
Email verification, password reset, organizer or admin sign-up, a sign-up
rate limit.

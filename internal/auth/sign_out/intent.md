# Intent: Sign out

`POST /api/sign-out` (no body, or `{}`). App code in `internal/auth`.

## Why
A user ends their sign-in session on this browser; the session must stop
working on the server, not just disappear from the browser.

## Who
Anyone; signed out it changes nothing on the server. Needs the CSRF token
like every state-changing request under `/api/` (F19).

## Inputs
- the `seatlane_auth` cookie, if any.

## Outputs
HTTP 200 `{"signed_out": true}`. The answer expires the `seatlane_auth`
cookie and sets a new CSRF cookie for a signed-out browser.

## Rule
The session whose token hash matches the cookie is deleted, so the same
token signs nobody in again even if it was copied. Signing out twice, or
without a session, answers the same.

## Failure cases
None.

## Out of scope
Signing out every device of an account.

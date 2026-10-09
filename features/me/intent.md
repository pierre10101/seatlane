# Intent: Me

## Why
The web app shows who is signed in (email and role) and offers Sign out, or
Sign in / Sign up when nobody is. It asks the server instead of guessing
from a cookie it cannot read (the sign-in cookie is HttpOnly).

## Who
Any signed-in user: `httpx.Roles("customer", "organizer", "admin")`. A
caller who is not signed in gets HTTP 401 `unauthorized` from the bridge-en
runtime before the action runs; the web app reads that as "signed out".
The user and role are the `user` and `role` inputs, set by the server from
Seatlane's sign-in session (the `seatlane_auth` cookie, looked up by
`internal/auth`); the caller never sends them (HTTP 400 if it does).

## Inputs
- `user` — the signed-in user's account id (set by the server).
- `role` — the signed-in user's role (set by the server).

## Outputs
- `user_id` — the account id (= `user`).
- `email` — the account's email, as stored (trimmed, lower case).
- `role` — the role the server checked for this request (= `role`).

## Rule
One read of the account row by its id; nothing is written. The password
hash is never read or returned.

## Failure cases
None.

## Out of scope
Changing email, password or role; deleting an account.

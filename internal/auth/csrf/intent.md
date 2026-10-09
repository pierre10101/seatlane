# Intent: CSRF protection

Middleware in `internal/auth` (`auth.CSRF`) around every route under
`/api/`. Not a feature of its own: it decides whether a state-changing
request reaches the feature at all.

## Why
The sign-in cookie is sent by the browser on every request to Seatlane,
including requests a hostile page makes. A hostile page must not be able to
hold, confirm or release seats, sign a user in or out, or sign up in the
user's name.

## Rule
Safe methods (GET, HEAD, OPTIONS) pass. Every other method under `/api/`
passes only if all of these hold:
1. if the request has an `Origin` header, its host equals the request's
   `Host`; and if it has `Sec-Fetch-Site`, that is `same-origin` or `none`;
2. it has an `X-CSRF-Token` header equal to its `seatlane_csrf` cookie
   (compared in constant time);
3. that token is one the server made for this browser's current sign-in:
   `<nonce>.<mac>`, where `nonce` is 128 random bits and `mac` is
   HMAC-SHA256 with the server's key over the nonce and the SHA-256 of the
   current `seatlane_auth` cookie (the empty text when signed out). A token
   from another sign-in, from before sign-in, or forged without the key fails.

The `seatlane_csrf` cookie (readable by the page's script, SameSite=Lax,
Path=/, Secure over TLS) is set when the web app's HTML is served, on any
`/api/` response whose request has no valid token, and anew by sign-up,
sign-in and sign-out, whose answers change the sign-in it is bound to. The
web app sends it back in `X-CSRF-Token`. The server key comes from
`SEATLANE_CSRF_KEY` (64 hex characters) or is random per process; after a
restart with a random key the first POST gets F19 together with a fresh
cookie, and the web app retries once.

## Failure cases
- F19: a state-changing request under `/api/` fails any of the three
  checks: HTTP 403, id F19, "missing or invalid CSRF token". The request
  does not reach the feature, so nothing is written; the answer carries a
  fresh `seatlane_csrf` cookie.

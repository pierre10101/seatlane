// Package auth is Seatlane's accounts and sign-in: sign-up, sign-in and
// sign-out, password hashing (bcrypt), sign-in sessions, the per-IP+email
// sign-in rate limit, CSRF protection, and the httpx.Identity hook that tells
// the bridge-en runtime who is signed in.
//
// It is app code outside features/ and outside the bridge-en grammar on
// purpose (bridge-en v0.3.0: "sign-in, password hashing and sessions stay in
// the app"). The bridge-en slices never see a password, a token or a cookie:
// they take the signed-in user as `server:"user"` (and `server:"role"`).
//
// Each endpoint has an intent.md in a sub-directory (sign_up/, sign_in/,
// sign_out/, csrf/) in the bridge-en I2 format; intent_test.go holds the
// code to it the way bridge-en -check holds a slice (I3): the F-IDs listed
// there are exactly the failures declared here, each with a TestF<n>_ check.
//
// Time is passed in: every rule takes `now` (unix seconds) as an argument;
// only the HTTP layer reads the server clock, through httpx.Now (which the
// dev clock and the tests replace).
package auth

import (
	"database/sql"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/internal/auth/db"
)

// Failure cases, app-wide F-IDs (docs/failures.md). They match the intent.md
// files under this directory (intent_test.go checks it).
var (
	F14 = failure.New("F14", http.StatusBadRequest, "email is not valid")
	F15 = failure.New("F15", http.StatusBadRequest, "password must be 10 to 72 bytes")
	F16 = failure.New("F16", http.StatusConflict, "email is already registered")
	F17 = failure.New("F17", http.StatusUnauthorized, "email or password is wrong")
	F18 = failure.New("F18", http.StatusTooManyRequests, "too many sign-in attempts, try again later")
	F19 = failure.New("F19", http.StatusForbidden, "missing or invalid CSRF token")
)

// Failures maps each endpoint (the intent.md directory) to the F-IDs it raises.
var Failures = map[string][]*failure.Failure{
	"sign_up":  {F14, F15, F16},
	"sign_in":  {F17, F18},
	"sign_out": {},
	"csrf":     {F19},
}

// Cookie names.
const (
	AuthCookie = "seatlane_auth" // the sign-in session token (HttpOnly)
	CSRFCookie = "seatlane_csrf" // the CSRF token, read by the page's script
	CSRFHeader = "X-CSRF-Token"  // where the page sends it back
)

// Routes of the auth endpoints.
const (
	SignUpRoute  = "POST /api/sign-up"
	SignInRoute  = "POST /api/sign-in"
	SignOutRoute = "POST /api/sign-out"
)

// SessionTTL is how long a sign-in session lasts: 7 days.
const SessionTTL int64 = 7 * 24 * 60 * 60

// RoleCustomer is the role sign-up gives every new account.
const RoleCustomer = "customer"

// Service is the accounts store and the HTTP endpoints over it.
type Service struct {
	q   *db.Queries
	key []byte // CSRF HMAC key
	// BcryptCost is the bcrypt work factor (12; tests lower it).
	BcryptCost int
}

// New returns the service over conn; key is the CSRF HMAC key (32 bytes).
func New(conn *sql.DB, key []byte) *Service {
	return &Service{q: db.New(txn.DB(conn)), key: key, BcryptCost: 12}
}

// now is the server clock in unix seconds, read only at the HTTP edge.
func now() int64 { return httpx.Now().Unix() }

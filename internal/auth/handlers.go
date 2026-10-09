package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/internal/auth/db"
)

// Account is the answer of sign-up and sign-in (never the password hash).
type Account struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// credentials is the body of sign-up and sign-in: exactly these two fields.
type credentials struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
}

const maxBody = 4 << 10

// decodeCredentials reads exactly one JSON object with exactly "email" and
// "password", both text; anything else is bad_request.
func decodeCredentials(r *http.Request) (email, password string, ok bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return "", "", false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var c credentials
	if err := dec.Decode(&c); err != nil || dec.More() || c.Email == nil || c.Password == nil {
		return "", "", false
	}
	if _, err := dec.Token(); err != io.EOF {
		return "", "", false
	}
	return *c.Email, *c.Password, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeFailure answers like the bridge-en runtime: {"error": {"id", "message"}}.
func writeFailure(w http.ResponseWriter, f *failure.Failure) {
	writeJSON(w, f.Status, httpx.ErrorBody{Error: httpx.ErrorDetail{ID: f.ID, Message: f.Message}})
}

func writeBadRequest(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, httpx.ErrorBody{Error: httpx.ErrorDetail{ID: httpx.BadInput.ID, Message: "the body must be one JSON object with exactly the text fields email and password"}})
}

func writeInternal(w http.ResponseWriter, err error) {
	log.Printf("auth: internal error: %v", err)
	writeJSON(w, http.StatusInternalServerError, httpx.ErrorBody{Error: httpx.ErrorDetail{ID: httpx.Internal.ID, Message: httpx.Internal.Message}})
}

// clientIP is the host part of the connection's remote address. Proxy
// headers (X-Forwarded-For) are not trusted.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// setAuthCookie starts the browser's sign-in with token and binds a new
// CSRF cookie to it. A session the browser had before is ended first.
func (s *Service) setAuthCookie(w http.ResponseWriter, r *http.Request, token string) {
	if old := authToken(r); old != "" && old != token {
		if err := s.SignOut(old); err != nil {
			log.Printf("auth: ending the previous session: %v", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: AuthCookie, Value: token, Path: "/", MaxAge: int(SessionTTL),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: overTLS(r),
	})
	s.SetCSRFCookie(w, r, token)
}

// startSession stores a new session for user at now and returns its token.
func (s *Service) startSession(ctx context.Context, userID, now int64) (string, error) {
	token := newSessionToken()
	err := s.q.CreateSession(ctx, db.CreateSessionParams{TokenHash: tokenHash(token), UserID: userID, Now: now, ExpiresAt: now + SessionTTL})
	return token, err
}

// SignUp creates a customer account and signs it in (sign_up/intent.md).
func (s *Service) SignUp(email, password string, now int64) (Account, string, error) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return Account{}, "", F14
	}
	if !ValidPassword(password) {
		return Account{}, "", F15
	}
	hash, err := hashPassword(password, s.BcryptCost)
	if err != nil {
		return Account{}, "", err
	}
	type result struct {
		acc   Account
		token string
	}
	res, err := txn.Run(context.Background(), func(ctx context.Context) (result, error) {
		row, err := s.q.CreateUser(ctx, db.CreateUserParams{Email: email, PasswordHash: hash, Role: RoleCustomer, Now: now})
		if errors.Is(err, sql.ErrNoRows) {
			return result{}, F16
		}
		if err != nil {
			return result{}, err
		}
		token, err := s.startSession(ctx, row.ID, now)
		if err != nil {
			return result{}, err
		}
		return result{Account{UserID: row.ID, Email: row.Email, Role: row.Role}, token}, nil
	})
	return res.acc, res.token, err
}

// SignIn checks both rate-limit caps (ip+email, and ip across all emails),
// then the password, and on success starts a new session
// (sign_in/intent.md). retryAfter is set with F18.
func (s *Service) SignIn(ip, email, password string, now int64) (acc Account, token string, retryAfter int64, err error) {
	email = NormalizeEmail(email)
	pairKey, ipKey := limitKey(ip, email), ipLimitKey(ip)
	type reservation struct {
		allowed    bool
		retryAfter int64
		ipCount    Attempts // the per-IP count as saved, for Refund
		user       db.UserByEmailRow
		found      bool
	}
	// One write transaction (BEGIN IMMEDIATE): read both counts, decide,
	// save both, before the password is checked.
	res, err := txn.Run(context.Background(), func(ctx context.Context) (reservation, error) {
		pair, err := s.attempts(ctx, pairKey)
		if err != nil {
			return reservation{}, err
		}
		ipPrev, err := s.attempts(ctx, ipKey)
		if err != nil {
			return reservation{}, err
		}
		nextPair, nextIP, allowed, wait := ReserveBoth(pair, ipPrev, now)
		if !allowed {
			return reservation{retryAfter: wait}, nil
		}
		if err := s.saveAttempts(ctx, pairKey, nextPair); err != nil {
			return reservation{}, err
		}
		if err := s.saveAttempts(ctx, ipKey, nextIP); err != nil {
			return reservation{}, err
		}
		user, err := s.q.UserByEmail(ctx, email)
		switch {
		case err == nil:
			return reservation{allowed: true, ipCount: nextIP, user: user, found: true}, nil
		case errors.Is(err, sql.ErrNoRows):
			return reservation{allowed: true, ipCount: nextIP}, nil
		}
		return reservation{}, err
	})
	if err != nil {
		return Account{}, "", 0, err
	}
	if !res.allowed {
		return Account{}, "", res.retryAfter, F18
	}
	// bcrypt runs outside any transaction: it takes ~0.25 s and must not
	// hold SQLite's write lock.
	if !passwordMatches(res.user.PasswordHash, res.found, password, s.BcryptCost) {
		return Account{}, "", 0, F17
	}
	token, err = txn.Run(context.Background(), func(ctx context.Context) (string, error) {
		if err := s.q.ClearSignInAttempts(ctx, pairKey); err != nil {
			return "", err
		}
		cur, err := s.attempts(ctx, ipKey)
		if err != nil {
			return "", err
		}
		if err := s.saveAttempts(ctx, ipKey, Refund(cur, res.ipCount)); err != nil {
			return "", err
		}
		if err := s.q.DeleteExpiredSessions(ctx, now); err != nil {
			return "", err
		}
		return s.startSession(ctx, res.user.ID, now)
	})
	if err != nil {
		return Account{}, "", 0, err
	}
	return Account{UserID: res.user.ID, Email: res.user.Email, Role: res.user.Role}, token, 0, nil
}

// attempts reads a rate-limit key's stored count (zero if none).
func (s *Service) attempts(ctx context.Context, key string) (Attempts, error) {
	row, err := s.q.SignInAttempts(ctx, key)
	switch {
	case err == nil:
		return Attempts{WindowStart: row.WindowStart, Failures: row.Failures}, nil
	case errors.Is(err, sql.ErrNoRows):
		return Attempts{}, nil
	}
	return Attempts{}, err
}

// saveAttempts stores a rate-limit key's count.
func (s *Service) saveAttempts(ctx context.Context, key string, a Attempts) error {
	return s.q.SaveSignInAttempts(ctx, db.SaveSignInAttemptsParams{Key: key, WindowStart: a.WindowStart, Failures: a.Failures})
}

// SignOut deletes the session of token (none: nothing to do).
func (s *Service) SignOut(token string) error {
	if token == "" {
		return nil
	}
	_, err := txn.Run(context.Background(), func(ctx context.Context) (struct{}, error) {
		return struct{}{}, s.q.DeleteSession(ctx, tokenHash(token))
	})
	return err
}

// Identify is the httpx.Identity hook: the signed-in user's id (digits) and
// role, from the seatlane_auth cookie's session while it has not expired.
func (s *Service) Identify(r *http.Request) (user, role string, ok bool) {
	token := authToken(r)
	if token == "" {
		return "", "", false
	}
	row, err := txn.Read(r.Context(), func(ctx context.Context) (db.SessionUserRow, error) {
		return s.q.SessionUser(ctx, db.SessionUserParams{TokenHash: tokenHash(token), Now: now()})
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("auth: identity lookup: %v", err)
		}
		return "", "", false
	}
	return strconv.FormatInt(row.ID, 10), row.Role, true
}

// HandleSignUp is POST /api/sign-up.
func (s *Service) HandleSignUp(w http.ResponseWriter, r *http.Request) {
	email, password, ok := decodeCredentials(r)
	if !ok {
		writeBadRequest(w)
		return
	}
	acc, token, err := s.SignUp(email, password, now())
	if s.answerFailure(w, err) {
		return
	}
	s.setAuthCookie(w, r, token)
	writeJSON(w, http.StatusCreated, acc)
}

// HandleSignIn is POST /api/sign-in.
func (s *Service) HandleSignIn(w http.ResponseWriter, r *http.Request) {
	email, password, ok := decodeCredentials(r)
	if !ok {
		writeBadRequest(w)
		return
	}
	acc, token, retryAfter, err := s.SignIn(clientIP(r), email, password, now())
	if errors.Is(err, F18) {
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
	}
	if s.answerFailure(w, err) {
		return
	}
	s.setAuthCookie(w, r, token)
	writeJSON(w, http.StatusOK, acc)
}

// HandleSignOut is POST /api/sign-out.
func (s *Service) HandleSignOut(w http.ResponseWriter, r *http.Request) {
	if err := s.SignOut(authToken(r)); err != nil {
		writeInternal(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: AuthCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: overTLS(r),
	})
	s.SetCSRFCookie(w, r, "")
	writeJSON(w, http.StatusOK, map[string]bool{"signed_out": true})
}

// answerFailure writes err (an F-ID failure, or a 500) and reports whether
// it wrote anything.
func (s *Service) answerFailure(w http.ResponseWriter, err error) bool {
	var f *failure.Failure
	switch {
	case err == nil:
		return false
	case errors.As(err, &f):
		writeFailure(w, f)
	default:
		writeInternal(w, err)
	}
	return true
}

// Mount adds the three endpoints to mux.
func (s *Service) Mount(mux *http.ServeMux) {
	mux.HandleFunc(SignUpRoute, s.HandleSignUp)
	mux.HandleFunc(SignInRoute, s.HandleSignIn)
	mux.HandleFunc(SignOutRoute, s.HandleSignOut)
}

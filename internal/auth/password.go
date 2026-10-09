package auth

import (
	"strings"
	"sync"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// Password length bounds in bytes; 72 is bcrypt's limit (longer passwords
// would be silently cut).
const (
	MinPassword = 10
	MaxPassword = 72
)

// MaxEmail is the longest email accepted (RFC 5321 path limit).
const MaxEmail = 254

// NormalizeEmail trims spaces and lower-cases: the form that is checked,
// stored and compared.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ValidEmail is the F14 rule on a normalized email: 1 to 254 characters,
// exactly one "@" with text on both sides, a "." after the "@", and no
// space or control character.
func ValidEmail(e string) bool {
	if e == "" || len(e) > MaxEmail || strings.Count(e, "@") != 1 {
		return false
	}
	for _, c := range e {
		if unicode.IsSpace(c) || unicode.IsControl(c) {
			return false
		}
	}
	local, domain, _ := strings.Cut(e, "@")
	return local != "" && domain != "" && strings.Contains(domain, ".")
}

// ValidPassword is the F15 rule: 10 to 72 bytes, both included.
func ValidPassword(p string) bool { return len(p) >= MinPassword && len(p) <= MaxPassword }

// dummyHashes holds, per bcrypt cost, a hash of a random password nobody
// knows. Sign-in compares against it for an email with no account, so an
// unknown email costs the same time as a wrong password (F17 answers both
// alike).
var dummyHashes sync.Map // int -> []byte

func dummyHash(cost int) []byte {
	if h, ok := dummyHashes.Load(cost); ok {
		return h.([]byte)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(randomText(32)), cost)
	if err != nil {
		panic(err)
	}
	v, _ := dummyHashes.LoadOrStore(cost, h)
	return v.([]byte)
}

func hashPassword(p string, cost int) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), cost)
	return string(h), err
}

// passwordMatches reports whether p matches hash; with ok false (no
// account) it still runs bcrypt against a dummy hash of the same cost and
// answers false.
func passwordMatches(hash string, ok bool, p string, cost int) bool {
	if !ok {
		_ = bcrypt.CompareHashAndPassword(dummyHash(cost), []byte(p))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"strings"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// randomText returns n random bytes from crypto/rand as lower-case base32.
func randomText(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return strings.ToLower(b32.EncodeToString(b))
}

// newSessionToken is 256 random bits (52 base32 characters).
func newSessionToken() string { return randomText(32) }

// tokenHash is what auth_sessions stores for a token: SHA-256, hex.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewKey returns a random 32-byte CSRF key.
func NewKey() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

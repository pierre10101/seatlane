package seed

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// DevEnv must be "1" for DevAccounts to run: the dev accounts are for a
// local database only, never a deployed one.
const DevEnv = "SEATLANE_DEV"

// DevPasswordEnv, when set, is the password the dev accounts get (10 to 72
// bytes); otherwise each run picks a random one and the server prints it.
const DevPasswordEnv = "SEATLANE_DEV_PASSWORD"

// DevAccount is one account the dev seed creates.
type DevAccount struct{ Email, Role string }

// DevAccounts are the dev seed's accounts. Customers sign up in the app.
// example.test is a reserved domain: no real mailbox has these addresses.
var DevAccounts = []DevAccount{
	{"organizer@example.test", "organizer"},
	{"admin@example.test", "admin"},
}

// ErrNotDev is the refusal when DevEnv is not "1".
var ErrNotDev = errors.New("refusing to create dev accounts: set " + DevEnv + "=1 (local development only; make seed does)")

// DevPassword is the password for the dev accounts: getenv(DevPasswordEnv)
// when set, else 16 random characters. It refuses unless getenv(DevEnv) is
// "1".
func DevPassword(getenv func(string) string) (string, error) {
	if getenv(DevEnv) != "1" {
		return "", ErrNotDev
	}
	if p := getenv(DevPasswordEnv); p != "" {
		if len(p) < 10 || len(p) > 72 {
			return "", fmt.Errorf("%s must be 10 to 72 bytes", DevPasswordEnv)
		}
		return p, nil
	}
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789" // no look-alikes
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

// LoadDevAccounts creates DevAccounts with password (bcrypt at cost), or,
// for an email that already has an account, resets its password and role.
// now is unix seconds.
func LoadDevAccounts(ctx context.Context, db *sql.DB, password string, cost int, now int64) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range DevAccounts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (email, password_hash, role, created_at) VALUES (?, ?, ?, ?)
			ON CONFLICT (email) DO UPDATE SET password_hash = excluded.password_hash, role = excluded.role`,
			a.Email, string(hash), a.Role, now); err != nil {
			return fmt.Errorf("seed account %s: %w", a.Email, err)
		}
	}
	return tx.Commit()
}

package seed_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierre10101/seatlane/internal/auth"
	"github.com/pierre10101/seatlane/internal/dbopen"
	"github.com/pierre10101/seatlane/internal/seed"
	"golang.org/x/crypto/bcrypt"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDevPasswordRefusesWithoutDevEnv(t *testing.T) {
	for _, v := range []string{"", "0", "true", "yes", " 1"} {
		if _, err := seed.DevPassword(env(map[string]string{seed.DevEnv: v, seed.DevPasswordEnv: "long-enough-pw"})); !errors.Is(err, seed.ErrNotDev) {
			t.Fatalf("%s=%q: got %v, want ErrNotDev", seed.DevEnv, v, err)
		}
	}
}

func TestDevPassword(t *testing.T) {
	p1, err := seed.DevPassword(env(map[string]string{seed.DevEnv: "1"}))
	if err != nil {
		t.Fatal(err)
	}
	p2, _ := seed.DevPassword(env(map[string]string{seed.DevEnv: "1"}))
	if len(p1) != 16 || !auth.ValidPassword(p1) || p1 == p2 {
		t.Fatalf("random passwords %q, %q", p1, p2)
	}
	p, err := seed.DevPassword(env(map[string]string{seed.DevEnv: "1", seed.DevPasswordEnv: "my-dev-password"}))
	if err != nil || p != "my-dev-password" {
		t.Fatalf("override: %q, %v", p, err)
	}
	for _, bad := range []string{"short", strings.Repeat("x", 73)} {
		if _, err := seed.DevPassword(env(map[string]string{seed.DevEnv: "1", seed.DevPasswordEnv: bad})); err == nil {
			t.Fatalf("%d-byte password accepted", len(bad))
		}
	}
}

// The dev accounts sign in with the printed password and have their roles;
// a second run resets the password (and role) instead of failing.
func TestLoadDevAccountsSignIn(t *testing.T) {
	ctx := context.Background()
	db, err := dbopen.Open(ctx, filepath.Join(t.TempDir(), "seatlane.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := seed.LoadDevAccounts(ctx, db, "first-password", bcrypt.MinCost, 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE users SET role = 'customer' WHERE email = 'admin@example.test'`); err != nil {
		t.Fatal(err)
	}
	if err := seed.LoadDevAccounts(ctx, db, "second-password", bcrypt.MinCost, 2000); err != nil {
		t.Fatal(err)
	}
	accounts := auth.New(db, auth.NewKey())
	accounts.BcryptCost = bcrypt.MinCost
	for _, a := range seed.DevAccounts {
		if _, _, _, err := accounts.SignIn("127.0.0.1", a.Email, "first-password", 3000); err == nil {
			t.Fatalf("%s: the old password still works", a.Email)
		}
		acc, _, _, err := accounts.SignIn("127.0.0.1", a.Email, "second-password", 3000)
		if err != nil {
			t.Fatalf("%s: %v", a.Email, err)
		}
		if acc.Role != a.Role {
			t.Fatalf("%s: role %q, want %q", a.Email, acc.Role, a.Role)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n != len(seed.DevAccounts) {
		t.Fatalf("users %d, %v", n, err)
	}
}

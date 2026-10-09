// Package checks holds the acceptance checks for Me, against a real SQLite
// database. Me has no failure case of its own; these checks prove who may
// call it (401 when not signed in, every app role allowed), that the user
// and role come only from the sign-in hook, and that the answer is the
// signed-in user's own account without its password hash.
package checks

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/txn"
	"github.com/pierre10101/seatlane/features/me"
	"github.com/pierre10101/seatlane/features/me/db"
	"github.com/pierre10101/seatlane/internal/testkit"
)

func TestMeAnswersTheSignedInAccount(t *testing.T) {
	conn := testkit.Open(t, 0)
	for _, u := range []struct {
		id          int64
		email, role string
	}{{testkit.Alice, "alice@example.com", "customer"}, {testkit.Olga, "olga@example.com", "organizer"}, {testkit.Ada, "ada@example.com", "admin"}} {
		testkit.Exec(t, conn, `INSERT INTO users (id, email, password_hash, role, created_at) VALUES (?, ?, 'secret-hash', ?, ?)`, u.id, u.email, u.role, testkit.T0)
	}
	a := me.New(db.New(txn.DB(conn)))
	out, err := txn.Read(context.Background(), func(ctx context.Context) (me.Output, error) {
		return a.Handle(ctx, me.Input{User: testkit.Olga, Role: "organizer"})
	})
	if err != nil || out != (me.Output{UserID: testkit.Olga, Email: "olga@example.com", Role: "organizer"}) {
		t.Fatalf("out %+v err %v", out, err)
	}

	mux := http.NewServeMux()
	mux.Handle(me.Route, httpx.Bind(me.Roles, a.Handle))
	h := testkit.Serve(mux)
	if rec := testkit.Do(h, http.MethodGet, "/api/me", "", "", ""); rec.Code != http.StatusUnauthorized || testkit.ErrorID(rec) != "unauthorized" {
		t.Fatalf("signed out: %d %s", rec.Code, rec.Body)
	}
	for _, q := range []string{"?user=4", "?role=admin"} {
		if rec := testkit.Do(h, http.MethodGet, "/api/me"+q, "", "1", "customer"); rec.Code != http.StatusBadRequest {
			t.Fatalf("caller-sent %s: %d %s", q, rec.Code, rec.Body)
		}
	}
	for _, c := range []struct{ user, role, email string }{{"1", "customer", "alice@example.com"}, {"3", "organizer", "olga@example.com"}, {"4", "admin", "ada@example.com"}} {
		rec := testkit.Do(h, http.MethodGet, "/api/me", "", c.user, c.role)
		var got me.Output
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Email != c.email || got.Role != c.role {
			t.Fatalf("%s %s: %d %s", c.user, c.role, rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "hash") {
			t.Fatalf("password hash in the answer: %s", rec.Body)
		}
	}
}

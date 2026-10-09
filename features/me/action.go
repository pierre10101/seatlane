// Package me is the Me slice: who is signed in. The why lives in intent.md;
// the English review rendering lives in me.en (generated, golden).
package me

import (
	"context"

	"github.com/pierre10101/go-ai-bridge/runtime/assert"
	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/seatlane/features/me/db"
)

// Route is the HTTP contract.
const Route = "GET /api/me"

// Roles: any signed-in user (401 if not signed in), checked by httpx.Bind
// before Handle runs.
var Roles = httpx.Roles("customer", "organizer", "admin")

// Input: the signed-in user and role, both set by the server.
type Input struct {
	User int64  `json:"user" server:"user"`
	Role string `json:"role" server:"role"`
}

// Output: the account, without its password hash.
type Output struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// Action reads the signed-in user's account.
type Action struct {
	q *db.Queries
}

// New wires the action.
func New(q *db.Queries) *Action { return &Action{q: q} }

// Handle reads one row; nothing is written.
func (a *Action) Handle(ctx context.Context, in Input) (Output, error) {
	account, err := a.q.AccountByID(ctx, in.User)
	if err != nil {
		return Output{}, err
	}

	out := Output{UserID: account.ID, Email: account.Email, Role: in.Role}
	assert.Post(out.UserID == in.User, "the answer is the signed-in user's own account")
	return out, nil
}

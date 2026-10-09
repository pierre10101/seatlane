// Package confirm_holds is the Confirm holds slice: every listed seat is sold
// to the user who holds it, or none is. The why lives in intent.md; the
// English review rendering lives in confirm_holds.en (generated, golden).
package confirm_holds

import (
	"context"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/assert"
	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/seatlane/features/confirm_holds/db"
)

// Route is the HTTP contract.
const Route = "POST /api/holds/confirm-all"

// Roles: only a signed-in customer may call it (401 if not signed in, 403
// for another role), checked by httpx.Bind before Handle runs.
var Roles = httpx.Roles("customer")

// Input: the seats on the review screen; the signed-in user and now are set by the server.
type Input struct {
	SeatIDs []int64 `json:"seat_ids" list:"1..20"`
	User    int64   `json:"user" server:"user"`
	Now     int64   `json:"now" clock:"now"`
}

// Output: how many seats were sold, when, and the server's clock.
type Output struct {
	Confirmed int64 `json:"confirmed"`
	SoldAt    int64 `json:"sold_at"`
	Now       int64 `json:"now"`
}

// Failure cases. IDs match intent.md, docs/failures.md and checks/.
var (
	F2  = failure.New("F2", http.StatusGone, "hold has expired")
	F13 = failure.New("F13", http.StatusConflict, "a seat in the list is not held by you")
)

// Action sells every listed seat to the user who holds it, or none.
type Action struct {
	q *db.Queries
}

// New wires the action.
func New(q *db.Queries) *Action { return &Action{q: q} }

// Handle sells the listed seats with one conditional UPDATE, explains an
// expired hold with a read made after it, and rolls everything back unless
// one row changed per listed seat.
func (a *Action) Handle(ctx context.Context, in Input) (Output, error) {
	confirmed, err := a.q.ConfirmSeats(ctx, db.ConfirmSeatsParams{User: in.User, Now: in.Now, SeatIds: in.SeatIDs})
	if err != nil {
		return Output{}, err
	}

	expired, err := a.q.CountExpiredHolds(ctx, db.CountExpiredHoldsParams{User: in.User, Now: in.Now, SeatIds: in.SeatIDs})
	if err != nil {
		return Output{}, err
	}
	if expired != 0 {
		return Output{}, F2
	}
	if confirmed != int64(len(in.SeatIDs)) {
		return Output{}, F13
	}

	sold, err := a.q.CountSoldNow(ctx, db.CountSoldNowParams{User: in.User, Now: in.Now, SeatIds: in.SeatIDs})
	if err != nil {
		return Output{}, err
	}

	out := Output{Confirmed: confirmed, SoldAt: in.Now, Now: in.Now}
	assert.Post(sold == confirmed, "every seat changed is sold to this user now")
	return out, nil
}

// Package confirm_hold is the Confirm hold slice. The why lives in intent.md;
// the English review rendering lives in confirm_hold.en (generated, golden).
package confirm_hold

import (
	"context"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/assert"
	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/seatlane/features/confirm_hold/db"
)

// Route is the HTTP contract.
const Route = "POST /api/holds/confirm"

// Roles: only a signed-in customer may call it (401 if not signed in, 403
// for another role), checked by httpx.Bind before Handle runs.
var Roles = httpx.Roles("customer")

// Input: the seat; the signed-in user and now are set by the server.
type Input struct {
	SeatID int64 `json:"seat_id"`
	User   int64 `json:"user" server:"user"`
	Now    int64 `json:"now" clock:"now"`
}

// Output: the sale, and the server's clock.
type Output struct {
	SeatID int64 `json:"seat_id"`
	SoldAt int64 `json:"sold_at"`
	Now    int64 `json:"now"`
}

// Failure cases. IDs match intent.md, docs/failures.md and checks/.
var (
	F2 = failure.New("F2", http.StatusGone, "hold has expired")
	F3 = failure.New("F3", http.StatusConflict, "there is no hold to confirm")
	F4 = failure.New("F4", http.StatusForbidden, "seat is held by someone else")
	F5 = failure.New("F5", http.StatusConflict, "seat is already confirmed")
	F6 = failure.New("F6", http.StatusConflict, "seat is already sold")
	F7 = failure.New("F7", http.StatusNotFound, "seat does not exist")
)

// Action sells a held seat to the user who holds it.
type Action struct {
	q *db.Queries
}

// New wires the action.
func New(q *db.Queries) *Action { return &Action{q: q} }

// Handle sells the seat with one conditional UPDATE, then explains a claim
// that changed nothing with reads made after it.
func (a *Action) Handle(ctx context.Context, in Input) (Output, error) {
	confirmed, err := a.q.ConfirmSeat(ctx, db.ConfirmSeatParams{User: in.User, Now: in.Now, SeatID: in.SeatID})
	if err != nil {
		return Output{}, err
	}

	seats, err := a.q.CountSeats(ctx, in.SeatID)
	if err != nil {
		return Output{}, err
	}
	if confirmed == 0 && seats == 0 {
		return Output{}, F7
	}

	seat, err := a.q.SeatHold(ctx, in.SeatID)
	if err != nil {
		return Output{}, err
	}
	if confirmed == 0 && seat.SoldTo == in.User {
		return Output{}, F5
	}
	if confirmed == 0 && seat.SoldTo != 0 {
		return Output{}, F6
	}
	if confirmed == 0 && seat.HeldBy == 0 {
		return Output{}, F3
	}
	if confirmed == 0 && seat.HeldBy == in.User {
		return Output{}, F2
	}
	if confirmed != 1 {
		return Output{}, F4
	}

	out := Output{SeatID: in.SeatID, SoldAt: seat.SoldAt, Now: in.Now}
	assert.Post(seat.SoldTo == in.User, "the seat is sold to the requester")
	assert.Post(out.SoldAt == in.Now, "the sale happened now")
	return out, nil
}

// Package hold_seat is the Hold seat slice. The why lives in intent.md; the
// English review rendering lives in hold_seat.en (generated, golden).
package hold_seat

import (
	"context"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/assert"
	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/seatlane/features/hold_seat/db"
)

// Route is the HTTP contract.
const Route = "POST /api/holds"

// Input: the seat; session and now are set by the server.
type Input struct {
	SeatID  int64 `json:"seat_id"`
	Session int64 `json:"session" server:"session"`
	Now     int64 `json:"now" clock:"now"`
}

// Output: the hold that was taken, and the server's clock for the countdown.
type Output struct {
	SeatID    int64 `json:"seat_id"`
	HeldAt    int64 `json:"held_at"`
	ExpiresAt int64 `json:"expires_at"`
	Now       int64 `json:"now"`
}

// Failure cases. IDs match intent.md, docs/failures.md and checks/.
var (
	F1 = failure.New("F1", http.StatusConflict, "seat is already held")
	F6 = failure.New("F6", http.StatusConflict, "seat is already sold")
	F7 = failure.New("F7", http.StatusNotFound, "seat does not exist")
	F8 = failure.New("F8", http.StatusUnauthorized, "session is required")
)

// Action holds one seat for one session.
type Action struct {
	q *db.Queries
}

// New wires the action.
func New(q *db.Queries) *Action { return &Action{q: q} }

// Handle claims the seat with one conditional UPDATE, then explains a claim
// that changed nothing with reads made after it.
func (a *Action) Handle(ctx context.Context, in Input) (Output, error) {
	if in.Session <= 0 {
		return Output{}, F8
	}

	claimed, err := a.q.HoldSeat(ctx, db.HoldSeatParams{Session: in.Session, Now: in.Now, SeatID: in.SeatID})
	if err != nil {
		return Output{}, err
	}

	seats, err := a.q.CountSeats(ctx, in.SeatID)
	if err != nil {
		return Output{}, err
	}
	if claimed == 0 && seats == 0 {
		return Output{}, F7
	}

	seat, err := a.q.SeatHold(ctx, in.SeatID)
	if err != nil {
		return Output{}, err
	}
	if claimed == 0 && seat.SoldTo != 0 {
		return Output{}, F6
	}
	if claimed != 1 {
		return Output{}, F1
	}

	out := Output{SeatID: in.SeatID, HeldAt: seat.HeldAt, ExpiresAt: seat.ExpiresAt, Now: in.Now}
	assert.Post(seat.HeldBy == in.Session, "the hold belongs to the requester")
	assert.Post(out.HeldAt == in.Now, "the hold was taken now")
	assert.Post(out.ExpiresAt > out.Now, "a new hold has not expired")
	return out, nil
}

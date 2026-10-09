// Package release_hold is the Release hold slice. The why lives in intent.md;
// the English review rendering lives in release_hold.en (generated, golden).
package release_hold

import (
	"context"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/assert"
	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/seatlane/features/release_hold/db"
)

// Route is the HTTP contract.
const Route = "POST /api/holds/release"

// Roles: only a signed-in customer may call it (401 if not signed in, 403
// for another role), checked by httpx.Bind before Handle runs.
var Roles = httpx.Roles("customer")

// Input: the seat; the signed-in user and now are set by the server.
type Input struct {
	SeatID int64 `json:"seat_id"`
	User   int64 `json:"user" server:"user"`
	Now    int64 `json:"now" clock:"now"`
}

// Output: the released seat, and the server's clock.
type Output struct {
	SeatID     int64 `json:"seat_id"`
	ReleasedAt int64 `json:"released_at"`
	Now        int64 `json:"now"`
}

// Failure cases. IDs match intent.md, docs/failures.md and checks/.
var (
	F5 = failure.New("F5", http.StatusConflict, "seat is already confirmed")
	F6 = failure.New("F6", http.StatusConflict, "seat is already sold")
	F7 = failure.New("F7", http.StatusNotFound, "seat does not exist")
	F9 = failure.New("F9", http.StatusConflict, "you have no hold on this seat")
)

// Action gives a held seat back.
type Action struct {
	q *db.Queries
}

// New wires the action.
func New(q *db.Queries) *Action { return &Action{q: q} }

// Handle clears the hold with one conditional UPDATE, then explains a claim
// that changed nothing with reads made after it.
func (a *Action) Handle(ctx context.Context, in Input) (Output, error) {
	released, err := a.q.ReleaseSeat(ctx, db.ReleaseSeatParams{SeatID: in.SeatID, User: in.User})
	if err != nil {
		return Output{}, err
	}

	seats, err := a.q.CountSeats(ctx, in.SeatID)
	if err != nil {
		return Output{}, err
	}
	if released == 0 && seats == 0 {
		return Output{}, F7
	}

	seat, err := a.q.SeatHold(ctx, in.SeatID)
	if err != nil {
		return Output{}, err
	}
	if released == 0 && seat.SoldTo == in.User {
		return Output{}, F5
	}
	if released == 0 && seat.SoldTo != 0 {
		return Output{}, F6
	}
	if released != 1 {
		return Output{}, F9
	}

	out := Output{SeatID: in.SeatID, ReleasedAt: in.Now, Now: in.Now}
	assert.Post(seat.HeldBy == 0, "nobody holds the seat any more")
	return out, nil
}

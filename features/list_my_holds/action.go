// Package list_my_holds is the List my holds slice. The why lives in
// intent.md; the English review rendering lives in list_my_holds.en
// (generated, golden).
package list_my_holds

import (
	"context"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/assert"
	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/page"
	"github.com/pierre10101/seatlane/features/list_my_holds/db"
	"github.com/pierre10101/seatlane/internal/domain"
)

// Route is the HTTP contract.
const Route = "GET /api/events/{id}/holds"

// Roles: only a signed-in customer may call it (401 if not signed in, 403
// for another role), checked by httpx.Bind before Handle runs.
var Roles = httpx.Roles("customer")

// Input: path and query; the signed-in user and now are set by the server.
type Input struct {
	EventID int64 `json:"event_id" path:"id"`
	After   int64 `json:"after" query:"after"`
	Limit   int64 `json:"limit" query:"limit"`
	User    int64 `json:"user" server:"user"`
	Now     int64 `json:"now" clock:"now"`
}

// Output: one page of the viewer's holds, and the server's clock for the countdown.
type Output struct {
	Holds     []domain.MyHold `json:"holds"`
	NextAfter int64           `json:"next_after"`
	Now       int64           `json:"now"`
}

// Failure cases. IDs match intent.md, docs/failures.md and checks/.
var (
	F11 = failure.New("F11", http.StatusBadRequest, "page limit is out of range")
	F12 = failure.New("F12", http.StatusBadRequest, "page cursor must be greater than zero")
)

// Action lists one user's holds.
type Action struct {
	q *db.Queries
}

// New wires the action.
func New(q *db.Queries) *Action { return &Action{q: q} }

// Handle reads only the seats this user holds.
func (a *Action) Handle(ctx context.Context, in Input) (Output, error) {
	if !page.IsPageLimit(in.Limit) {
		return Output{}, F11
	}
	if in.After <= 0 {
		return Output{}, F12
	}

	rows, err := a.q.ListMyHolds(ctx, db.ListMyHoldsParams{EventID: in.EventID, User: in.User, After: in.After, Limit: in.Limit})
	if err != nil {
		return Output{}, err
	}

	items := make([]domain.MyHold, len(rows))
	for i, row := range rows {
		items[i] = domain.MyHold{
			SeatID:    row.ID,
			HeldAt:    row.HeldAt,
			ExpiresAt: row.ExpiresAt,
			Active:    domain.IsActiveHold(row.ExpiresAt, in.Now),
		}
	}

	next := page.NextAfter(rows, "id", in.Limit)

	out := Output{Holds: items, NextAfter: next, Now: in.Now}
	assert.Post(out.Holds != nil, "an empty page is [], not null")
	return out, nil
}

// Package list_events is the List events slice. The why lives in intent.md;
// the English review rendering lives in list_events.en (generated, golden).
package list_events

import (
	"context"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/assert"
	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/go-ai-bridge/runtime/page"
	"github.com/pierre10101/seatlane/features/list_events/db"
	"github.com/pierre10101/seatlane/internal/domain"
)

// Route is the HTTP contract.
const Route = "GET /api/events"

// Input: keyset paging over event ids.
type Input struct {
	After int64 `json:"after" query:"after"`
	Limit int64 `json:"limit" query:"limit"`
}

// Output: one page of event cards and the next cursor.
type Output struct {
	Events    []domain.EventCard `json:"events"`
	NextAfter int64              `json:"next_after"`
}

// Failure cases. IDs match intent.md, docs/failures.md and checks/.
var (
	F11 = failure.New("F11", http.StatusBadRequest, "page limit is out of range")
	F12 = failure.New("F12", http.StatusBadRequest, "page cursor must be greater than zero")
)

// Action lists one page of events on sale.
type Action struct {
	q *db.Queries
}

// New wires the action.
func New(q *db.Queries) *Action { return &Action{q: q} }

// Handle runs in one read-only transaction (httpx.Bind on a GET).
func (a *Action) Handle(ctx context.Context, in Input) (Output, error) {
	if !page.IsPageLimit(in.Limit) {
		return Output{}, F11
	}
	if in.After <= 0 {
		return Output{}, F12
	}

	rows, err := a.q.ListEvents(ctx, db.ListEventsParams{After: in.After, Limit: in.Limit})
	if err != nil {
		return Output{}, err
	}

	items := make([]domain.EventCard, len(rows))
	for i, row := range rows {
		items[i] = domain.EventCard{
			EventID:   row.ID,
			Name:      row.Name,
			Venue:     row.Venue,
			City:      row.City,
			StartsAt:  row.StartsAt,
			Tagline:   row.Tagline,
			FromPrice: domain.Money{Cents: row.FromPriceCents, Currency: row.Currency},
			ToPrice:   domain.Money{Cents: row.ToPriceCents, Currency: row.Currency},
		}
	}

	next := page.NextAfter(rows, "id", in.Limit)

	out := Output{Events: items, NextAfter: next}
	assert.Post(out.Events != nil, "an empty page is [], not null")
	return out, nil
}

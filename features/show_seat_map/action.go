// Package show_seat_map is the Show seat map slice. The why lives in
// intent.md; the English review rendering lives in show_seat_map.en.
package show_seat_map

import (
	"context"
	"net/http"

	"github.com/pierre10101/go-ai-bridge/runtime/assert"
	"github.com/pierre10101/go-ai-bridge/runtime/failure"
	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
	"github.com/pierre10101/go-ai-bridge/runtime/page"
	"github.com/pierre10101/seatlane/features/show_seat_map/db"
	"github.com/pierre10101/seatlane/internal/domain"
)

// Route is the HTTP contract.
const Route = "GET /api/events/{id}/seats"

// Roles: anyone, signed in or not.
var Roles = httpx.Public

// Input: path and query; the signed-in user and now are set by the server.
type Input struct {
	EventID int64 `json:"event_id" path:"id"`
	After   int64 `json:"after" query:"after"`
	Limit   int64 `json:"limit" query:"limit"`
	User    int64 `json:"user" server:"user"`
	Now     int64 `json:"now" clock:"now"`
}

// Output: the event, one page of seats with their state for the viewer,
// the next cursor and the server's clock.
type Output struct {
	Event     domain.EventCard  `json:"event"`
	Seats     []domain.SeatView `json:"seats"`
	NextAfter int64             `json:"next_after"`
	Now       int64             `json:"now"`
}

// Failure cases. IDs match intent.md, docs/failures.md and checks/.
var (
	F10 = failure.New("F10", http.StatusNotFound, "event does not exist")
	F11 = failure.New("F11", http.StatusBadRequest, "page limit is out of range")
	F12 = failure.New("F12", http.StatusBadRequest, "page cursor must be greater than zero")
)

// Action lists one page of an event's seats.
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

	events, err := a.q.CountEvents(ctx, in.EventID)
	if err != nil {
		return Output{}, err
	}
	if events == 0 {
		return Output{}, F10
	}

	event, err := a.q.EventByID(ctx, in.EventID)
	if err != nil {
		return Output{}, err
	}

	rows, err := a.q.ListEventSeats(ctx, db.ListEventSeatsParams{EventID: in.EventID, After: in.After, Limit: in.Limit})
	if err != nil {
		return Output{}, err
	}

	items := make([]domain.SeatView, len(rows))
	for i, row := range rows {
		items[i] = domain.SeatView{
			SeatID:      row.ID,
			Section:     row.Section,
			SectionRank: row.SectionRank,
			Row:         row.RowLabel,
			Number:      row.SeatNumber,
			Price:       domain.Money{Cents: row.PriceCents, Currency: event.Currency},
			Available:   domain.IsAvailable(row.HeldBy, row.ExpiresAt, row.SoldTo, in.Now),
			HeldByMe:    domain.IsHeldBy(row.HeldBy, row.ExpiresAt, row.SoldTo, in.User, in.Now),
			HeldByOther: domain.IsHeldByOther(row.HeldBy, row.ExpiresAt, row.SoldTo, in.User, in.Now),
			SoldToMe:    domain.IsSoldTo(row.SoldTo, in.User),
			SoldToOther: domain.IsSoldToOther(row.SoldTo, in.User),
		}
	}

	next := page.NextAfter(rows, "id", in.Limit)

	card := domain.EventCard{
		EventID:   event.ID,
		Name:      event.Name,
		Venue:     event.Venue,
		City:      event.City,
		StartsAt:  event.StartsAt,
		Tagline:   event.Tagline,
		FromPrice: domain.Money{Cents: event.FromPriceCents, Currency: event.Currency},
		ToPrice:   domain.Money{Cents: event.ToPriceCents, Currency: event.Currency},
	}
	out := Output{Event: card, Seats: items, NextAfter: next, Now: in.Now}
	assert.Post(out.Seats != nil, "an empty page is [], not null")
	return out, nil
}

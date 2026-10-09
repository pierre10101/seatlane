// Package seed fills an empty database with demo events and seat grids.
// It is plumbing outside features/: plain SQL on the *sql.DB at startup.
package seed

import (
	"context"
	"database/sql"
	"fmt"
)

// Section is one block of rows with one price.
type Section struct {
	Name       string
	Rows       []string
	SeatsInRow int
	PriceCents int64
}

// Event is one demo event and its venue layout (sections nearest the stage first).
type Event struct {
	Name, Venue, City, Tagline string
	StartsAt                   int64 // unix seconds
	Sections                   []Section
}

// Events are the demo events. Prices are integer cents (ZAR).
var Events = []Event{
	{
		Name: "Nocturnes by Candlelight", Venue: "Artscape Opera House", City: "Cape Town",
		Tagline:  "Chopin and Satie by candlelight with the Cape Town Philharmonic strings.",
		StartsAt: 1794596400, // 2026-11-14 19:00 SAST
		Sections: []Section{
			{"Stalls", []string{"A", "B", "C", "D", "E", "F"}, 16, 65000},
			{"Dress Circle", []string{"G", "H", "J", "K"}, 14, 48000},
			{"Upper Circle", []string{"L", "M", "N"}, 12, 32000},
		},
	},
	{
		Name: "Highveld Jazz Nights", Venue: "Joburg Theatre, Mandela Stage", City: "Johannesburg",
		Tagline:  "A big-band evening of township jazz and swing standards.",
		StartsAt: 1795633200, // 2026-11-26 20:00 SAST
		Sections: []Section{
			{"Front Stalls", []string{"A", "B", "C", "D"}, 18, 55000},
			{"Rear Stalls", []string{"E", "F", "G", "H", "J"}, 18, 42000},
			{"Balcony", []string{"K", "L", "M"}, 14, 28000},
		},
	},
	{
		Name: "The Long Table: A Comedy Special", Venue: "Durban Playhouse, Drama Theatre", City: "Durban",
		Tagline:  "Five comics, one dinner table and absolutely no seating plan. Except this one.",
		StartsAt: 1796929200, // 2026-12-11 20:00 SAST
		Sections: []Section{
			{"Orchestra", []string{"A", "B", "C", "D", "E"}, 12, 38000},
			{"Mezzanine", []string{"F", "G", "H"}, 10, 26000},
		},
	},
}

// IfEmpty seeds the demo data when the events table is empty.
func IfEmpty(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&n); err != nil || n > 0 {
		return err
	}
	return Load(ctx, db, Events)
}

// Load inserts events and their seats in one transaction.
func Load(ctx context.Context, db *sql.DB, events []Event) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, e := range events {
		from, to := e.Sections[0].PriceCents, e.Sections[0].PriceCents
		for _, s := range e.Sections {
			from, to = min(from, s.PriceCents), max(to, s.PriceCents)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO events (name, venue, city, starts_at, tagline, from_price_cents, to_price_cents, currency, on_sale)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'ZAR', 1)`, e.Name, e.Venue, e.City, e.StartsAt, e.Tagline, from, to)
		if err != nil {
			return fmt.Errorf("seed event %q: %w", e.Name, err)
		}
		eventID, _ := res.LastInsertId()
		for rank, s := range e.Sections {
			for _, row := range s.Rows {
				for num := 1; num <= s.SeatsInRow; num++ {
					if _, err := tx.ExecContext(ctx, `INSERT INTO seats (event_id, section, section_rank, row_label, seat_number, price_cents)
						VALUES (?, ?, ?, ?, ?, ?)`, eventID, s.Name, rank+1, row, num, s.PriceCents); err != nil {
						return fmt.Errorf("seed seat: %w", err)
					}
				}
			}
		}
	}
	return tx.Commit()
}

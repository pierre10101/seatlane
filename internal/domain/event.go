package domain

// EventCard is one event as the event list and the seat map show it.
//
// bridge-en: an event card
type EventCard struct {
	EventID   int64  `json:"event_id"`
	Name      string `json:"name"`
	Venue     string `json:"venue"`
	City      string `json:"city"`
	StartsAt  int64  `json:"starts_at"`
	Tagline   string `json:"tagline"`
	FromPrice Money  `json:"from_price"`
	ToPrice   Money  `json:"to_price"`
}

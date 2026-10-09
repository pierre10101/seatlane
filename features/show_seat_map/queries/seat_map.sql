-- name: CountEvents :one
SELECT COUNT(*) FROM events WHERE id = ?;

-- name: EventByID :one
SELECT id, name, venue, city, starts_at, tagline, from_price_cents, currency FROM events WHERE id = ?;

-- name: ListEventSeats :many
SELECT id, section, section_rank, row_label, seat_number, price_cents, held_by, expires_at, sold_to
FROM seats
WHERE event_id = sqlc.arg(event_id) AND id < sqlc.arg(after)
ORDER BY id DESC
LIMIT sqlc.arg(limit);

-- name: ListEvents :many
SELECT id, name, venue, city, starts_at, tagline, from_price_cents, to_price_cents, currency
FROM events
WHERE on_sale = 1 AND id < sqlc.arg(after)
ORDER BY id DESC
LIMIT sqlc.arg(limit);

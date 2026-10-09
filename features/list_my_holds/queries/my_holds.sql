-- The viewer's own holds on one event, newest seat first (Q5): only rows
-- whose held_by is this user, so nobody else's hold time is ever read
-- out. Expired holds of this user are listed too, flagged inactive.
-- name: ListMyHolds :many
SELECT id, held_at, expires_at
FROM seats
WHERE event_id = sqlc.arg(event_id) AND held_by = sqlc.arg(user) AND sold_to = 0 AND id < sqlc.arg(after)
ORDER BY id DESC
LIMIT sqlc.arg(limit);

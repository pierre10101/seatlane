-- One statement holds the seat only if, at that moment, it is not sold and
-- nobody holds it or its hold has expired (Q6). The hold lasts 600 seconds.
-- The caller checks that exactly one row changed (S10).
-- name: HoldSeat :execrows
UPDATE seats
SET held_by = sqlc.arg(user), held_at = sqlc.arg(now), expires_at = sqlc.arg(now) + 600
WHERE id = sqlc.arg(seat_id) AND sold_to = 0 AND (held_by = 0 OR expires_at <= sqlc.arg(now));

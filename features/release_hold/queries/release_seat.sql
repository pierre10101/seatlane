-- One statement clears the hold only if, at that moment, this session holds
-- the seat and it is not sold (Q6 + S10).
-- name: ReleaseSeat :execrows
UPDATE seats
SET held_by = '', held_at = 0, expires_at = 0
WHERE id = sqlc.arg(seat_id) AND held_by = sqlc.arg(session) AND sold_to = '';

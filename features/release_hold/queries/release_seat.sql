-- One statement clears the hold only if, at that moment, this user holds
-- the seat and it is not sold (Q6 + S10).
-- name: ReleaseSeat :execrows
UPDATE seats
SET held_by = 0, held_at = 0, expires_at = 0
WHERE id = sqlc.arg(seat_id) AND held_by = sqlc.arg(user) AND sold_to = 0;

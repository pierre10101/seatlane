-- One statement sells the seat only if, at that moment, it is not sold, this
-- session holds it and the hold has not expired (Q6 + S10).
-- name: ConfirmSeat :execrows
UPDATE seats
SET sold_to = sqlc.arg(session), sold_at = sqlc.arg(now)
WHERE id = sqlc.arg(seat_id) AND held_by = sqlc.arg(session) AND sold_to = '' AND expires_at > sqlc.arg(now);

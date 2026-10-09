-- One statement sells every listed seat only if, at that moment, it is not
-- sold, this session holds it and the hold has not expired (Q6). The IN list
-- is on the table's key and comes last (Q7), so the action can check that one
-- row changed per listed seat (S11): all seats or none.
-- name: ConfirmSeats :execrows
UPDATE seats
SET sold_to = sqlc.arg(session), sold_at = sqlc.arg(now)
WHERE held_by = sqlc.arg(session) AND sold_to = '' AND expires_at > sqlc.arg(now) AND id IN (sqlc.slice(seat_ids));

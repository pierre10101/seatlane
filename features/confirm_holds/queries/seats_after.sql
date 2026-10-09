-- Reads after the claim (W1 allows them), in the same transaction. The claim
-- has just sold every listed seat this session held with a live hold, so a
-- listed seat this session still holds and that is still unsold is one whose
-- hold has expired (expires_at is now or earlier): F2. The sold count is the
-- postcondition.
-- name: CountExpiredHolds :one
SELECT COUNT(*) FROM seats WHERE held_by = sqlc.arg(session) AND sold_to = '' AND id IN (sqlc.slice(seat_ids));

-- name: CountSoldNow :one
SELECT COUNT(*) FROM seats WHERE sold_to = sqlc.arg(session) AND sold_at = sqlc.arg(now) AND id IN (sqlc.slice(seat_ids));

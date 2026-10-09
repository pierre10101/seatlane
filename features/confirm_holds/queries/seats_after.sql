-- Reads after the claim (W1 allows them), in the same transaction. A listed
-- seat this session still holds, still unsold, whose hold ended no later than
-- now has expired: F2. The read compares with the server-set now (Q1), before
-- the IN list, which stays last (Q7). A listed seat whose expired hold was
-- taken by another session is not held by this session, so it is not counted
-- here and the action answers F13. The sold count is the postcondition.
-- name: CountExpiredHolds :one
SELECT COUNT(*) FROM seats WHERE held_by = sqlc.arg(session) AND sold_to = '' AND expires_at <= sqlc.arg(now) AND id IN (sqlc.slice(seat_ids));

-- name: CountSoldNow :one
SELECT COUNT(*) FROM seats WHERE sold_to = sqlc.arg(session) AND sold_at = sqlc.arg(now) AND id IN (sqlc.slice(seat_ids));

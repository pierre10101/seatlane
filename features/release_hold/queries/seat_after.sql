-- Reads after the claim (W1 allows them): they explain a claim that changed
-- nothing, and give back the hold that was taken.
-- name: CountSeats :one
SELECT COUNT(*) FROM seats WHERE id = ?;

-- name: SeatHold :one
SELECT held_by, sold_to FROM seats WHERE id = ?;

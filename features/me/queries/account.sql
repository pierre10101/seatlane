-- The signed-in user's own account row, by the server-set user id (Q2).
-- The password hash is never read.
-- name: AccountByID :one
SELECT id, email, role FROM users WHERE id = ?;

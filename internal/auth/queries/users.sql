-- Writes only if no account has this email: two sign-ups with one email
-- cannot both succeed (no rows back = the email is taken).
-- name: CreateUser :one
INSERT INTO users (email, password_hash, role, created_at)
VALUES (sqlc.arg(email), sqlc.arg(password_hash), sqlc.arg(role), sqlc.arg(now))
ON CONFLICT (email) DO NOTHING
RETURNING id, email, role;

-- name: UserByEmail :one
SELECT id, email, role, password_hash FROM users WHERE email = ?;

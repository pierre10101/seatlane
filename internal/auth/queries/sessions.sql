-- name: CreateSession :exec
INSERT INTO auth_sessions (token_hash, user_id, created_at, expires_at)
VALUES (sqlc.arg(token_hash), sqlc.arg(user_id), sqlc.arg(now), sqlc.arg(expires_at));

-- The identity hook: a session counts while expires_at is later than now.
-- name: SessionUser :one
SELECT users.id, users.role
FROM auth_sessions JOIN users ON users.id = auth_sessions.user_id
WHERE auth_sessions.token_hash = sqlc.arg(token_hash) AND auth_sessions.expires_at > sqlc.arg(now);

-- name: DeleteSession :exec
DELETE FROM auth_sessions WHERE token_hash = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM auth_sessions WHERE expires_at <= sqlc.arg(now);

-- name: SignInAttempts :one
SELECT window_start, failures FROM sign_in_attempts WHERE key = ?;

-- name: SaveSignInAttempts :exec
INSERT INTO sign_in_attempts (key, window_start, failures)
VALUES (sqlc.arg(key), sqlc.arg(window_start), sqlc.arg(failures))
ON CONFLICT (key) DO UPDATE SET window_start = excluded.window_start, failures = excluded.failures;

-- name: ClearSignInAttempts :exec
DELETE FROM sign_in_attempts WHERE key = ?;

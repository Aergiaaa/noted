-- name: GetSessionByTokenHash :one
SELECT *
FROM sessions
WHERE token_hash = ?;

-- name: CreateSession :exec
INSERT INTO sessions (id, token_hash, created_at, expires_at, last_seen)
VALUES (?, ?, ?, ?, ?);

-- name: TouchSession :exec
UPDATE sessions
SET last_seen  = ?,
    expires_at = ?
WHERE id = ?;

-- name: DeleteSession :exec
DELETE
FROM sessions
WHERE id = ?;

-- name: DeleteSessionByTokenHash :exec
DELETE
FROM sessions
WHERE token_hash = ?;

-- name: DeleteExpiredSessions :exec
DELETE
FROM sessions
WHERE expires_at < ?;

-- name: DeleteAllSessions :exec
DELETE
FROM sessions;

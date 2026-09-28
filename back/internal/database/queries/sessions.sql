-- name: GetSessionByTokenHash :one
-- Lookup by SHA-256 of the opaque cookie token; token_hash is UNIQUE.
SELECT *
FROM sessions
WHERE token_hash = ?;

-- name: CreateSession :exec
INSERT INTO sessions (id, token_hash, created_at, expires_at, last_seen)
VALUES (?, ?, ?, ?, ?);

-- name: TouchSession :exec
-- Sliding expiry: bump last_seen and extends expires_at (F4 computes both).
UPDATE sessions
SET last_seen  = ?,
    expires_at = ?
WHERE id = ?;

-- name: DeleteSession :exec
DELETE
FROM sessions
WHERE id = ?;

-- name: DeleteExpiredSessions :exec
-- Purge on login/periodically: absolute cap already passed.
DELETE
FROM sessions
WHERE expires_at < ?;

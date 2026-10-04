-- name: GetEnrollment :one
-- Row exists only between `server enroll` commit and `reset-auth`.
SELECT *
FROM enrollment
WHERE id = 1;

-- name: CreateEnrollment :exec
-- Single-row upsert shape: id is always 1, PK rejects a second row.
INSERT INTO enrollment (id, totp_secret, enrolled_at, updated_at)
VALUES (1, ?, ?, ?);

-- name: ClearEnrollment :exec
-- reset-auth wipes enrollment so `server enroll` can run again.
DELETE
FROM enrollment
WHERE id = 1;

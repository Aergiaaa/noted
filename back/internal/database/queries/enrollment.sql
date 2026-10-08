-- name: GetEnrollment :one
SELECT *
FROM enrollment
WHERE id = 1;

-- name: CreateEnrollment :exec
INSERT INTO enrollment (id, totp_secret, enrolled_at, updated_at)
VALUES (1, ?, ?, ?);

-- name: ClearEnrollment :exec
DELETE
FROM enrollment
WHERE id = 1;

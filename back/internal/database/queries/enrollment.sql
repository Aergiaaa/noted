-- name: GetEnrollment :one
SELECT *
FROM enrollment
WHERE id = 1;

-- name: CreateEnrollment :execrows
-- ON CONFLICT (id) DO NOTHING makes the single-row table itself the
-- arbiter of "already enrolled": zero affected rows means a second
-- enroll, so no SQLite error-text matching is needed.
INSERT INTO enrollment (id, totp_secret, enrolled_at, updated_at)
VALUES (1, ?, ?, ?)
ON CONFLICT (id) DO NOTHING;

-- name: ClearEnrollment :exec
DELETE
FROM enrollment
WHERE id = 1;

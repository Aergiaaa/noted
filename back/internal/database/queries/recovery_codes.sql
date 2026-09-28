-- name: GetRecoveryCodeByHash :one
-- Single-use recovery login: bcrypt hash is UNIQUE, lookup by hash.
SELECT *
FROM recovery_codes
WHERE code_hash = ?;

-- name: ListUnusedRecoveryCodes :many
-- Enroll shows N codes once; login marks them used one at a time.
SELECT *
FROM recovery_codes
WHERE used_at IS NULL
ORDER BY created_at, id;

-- name: CreateRecoveryCode :exec
INSERT INTO recovery_codes (id, code_hash, used_at, created_at)
VALUES (?, ?, ?, ?);

-- name: MarkRecoveryCodeUsed :exec
UPDATE recovery_codes
SET used_at = ?
WHERE id = ?;

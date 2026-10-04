-- name: MarkAllRecoveryCodesUsed :exec
-- Rotation (POST /api/auth/recovery-codes) burns the whole unused pool
-- before fresh codes are minted, so a leaked code dies with its siblings.
UPDATE recovery_codes
SET used_at = ?
WHERE used_at IS NULL;

-- name: DeleteAllRecoveryCodes :exec
-- reset-auth clears the pool together with enrollment + sessions.
DELETE
FROM recovery_codes;

-- name: ListUnusedRecoveryCodes :many
-- Login scans the unused set (bcrypt hashes are salted, so no direct lookup).
SELECT *
FROM recovery_codes
WHERE used_at IS NULL
ORDER BY created_at, id;

-- name: CreateRecoveryCode :exec
INSERT INTO recovery_codes (id, code_hash, used_at, created_at)
VALUES (?, ?, ?, ?);

-- name: MarkRecoveryCodeUsed :execrows
-- Conditional burn: only a row still unused is marked, and the caller gets
-- the affected-row count so a racing double-use loses cleanly (0 rows).
UPDATE recovery_codes
SET used_at = ?
WHERE id = ?
  AND used_at IS NULL;

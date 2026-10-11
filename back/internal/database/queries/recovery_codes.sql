-- name: MarkAllRecoveryCodesUsed :exec
-- Burns the whole unused pool before fresh codes are minted, so a leaked
-- code dies with its siblings.
UPDATE recovery_codes
SET used_at = ?
WHERE used_at IS NULL;

-- name: DeleteAllRecoveryCodes :exec
DELETE
FROM recovery_codes;

-- name: ListUnusedRecoveryCodes :many
-- Salted bcrypt hashes cannot be looked up directly, so login scans.
SELECT *
FROM recovery_codes
WHERE used_at IS NULL
ORDER BY created_at, id;

-- name: CreateRecoveryCode :exec
INSERT INTO recovery_codes (id, code_hash, used_at, created_at)
VALUES (?, ?, ?, ?);

-- name: MarkRecoveryCodeUsed :execrows
-- Conditional burn: the affected-row count lets a racing double-use lose
-- cleanly (0 rows).
UPDATE recovery_codes
SET used_at = ?
WHERE id = ?
  AND used_at IS NULL;

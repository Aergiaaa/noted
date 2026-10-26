-- name: ListWallets :many
-- Active wallets by default; include_archived non-nil pulls archived ones
-- too (they keep history and balances, DOMAIN.md).
SELECT *
FROM wallets
WHERE sqlc.narg(include_archived) IS NOT NULL OR archived_at IS NULL
ORDER BY archived_at IS NOT NULL, name;

-- name: GetWallet :one
SELECT *
FROM wallets
WHERE id = ?;

-- name: CreateWallet :one
INSERT INTO wallets (id, name, archived_at, created_at, updated_at)
VALUES (?, ?, NULL, ?, ?)
RETURNING *;

-- name: UpdateWallet :one
-- Full-column update; archived_at nil un-archives.
UPDATE wallets
SET name        = ?,
    archived_at = ?,
    updated_at  = ?
WHERE id = ?
RETURNING *;

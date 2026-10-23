-- name: BalanceSums :one
-- Derived balance: no stored total anywhere (ARCHITECTURE.md D4).
-- Soft-deleted (voided) transactions drop out of the sums on read.
-- Transfers cancel here (income leg in, expense leg out of the total);
-- per-wallet splits live in BalanceSumsByWallet + TransferSumsByWallet.
SELECT CAST(COALESCE(SUM(CASE WHEN kind = 'income' THEN amount_cents ELSE 0 END), 0) AS INTEGER)  AS income_cents,
       CAST(COALESCE(SUM(CASE WHEN kind = 'expense' THEN amount_cents ELSE 0 END), 0) AS INTEGER) AS expense_cents
FROM transactions
WHERE deleted_at IS NULL;

-- name: BalanceSumsByWallet :many
-- Per-wallet income/expense (income/expense rows only; transfer deltas
-- come from the two Transfer* queries below).
SELECT wallet_id,
       CAST(COALESCE(SUM(CASE WHEN kind = 'income' THEN amount_cents ELSE 0 END), 0) AS INTEGER)  AS income_cents,
       CAST(COALESCE(SUM(CASE WHEN kind = 'expense' THEN amount_cents ELSE 0 END), 0) AS INTEGER) AS expense_cents
FROM transactions
WHERE deleted_at IS NULL
  AND wallet_id IS NOT NULL
GROUP BY wallet_id;

-- name: TransferInSumsByWallet :many
SELECT to_wallet_id AS wallet_id,
       CAST(COALESCE(SUM(amount_cents), 0) AS INTEGER) AS amount_cents
FROM transactions
WHERE deleted_at IS NULL
  AND kind = 'transfer'
  AND to_wallet_id IS NOT NULL
GROUP BY to_wallet_id;

-- name: TransferOutSumsByWallet :many
SELECT from_wallet_id AS wallet_id,
       CAST(COALESCE(SUM(amount_cents), 0) AS INTEGER) AS amount_cents
FROM transactions
WHERE deleted_at IS NULL
  AND kind = 'transfer'
  AND from_wallet_id IS NOT NULL
GROUP BY from_wallet_id;

-- name: ListTransactionsPage :many
-- Keyset history, newest first; cursor filters are nil on the first page.
-- wallet_id matches either side of a transfer (from/to).
SELECT *
FROM transactions
WHERE deleted_at IS NULL
  AND (sqlc.narg(cursor_occurred_on) IS NULL
       OR occurred_on < sqlc.narg(cursor_occurred_on)
       OR (occurred_on = sqlc.narg(cursor_occurred_on)
           AND id < sqlc.narg(cursor_id)))
  AND (sqlc.narg(wallet_id) IS NULL
       OR wallet_id = sqlc.narg(wallet_id)
       OR from_wallet_id = sqlc.narg(wallet_id)
       OR to_wallet_id = sqlc.narg(wallet_id))
ORDER BY occurred_on DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetTransaction :one
SELECT *
FROM transactions
WHERE id = ?
  AND deleted_at IS NULL;

-- name: CreateTransaction :one
INSERT INTO transactions (id, kind, amount_cents, currency, occurred_on, memo,
                          wallet_id, from_wallet_id, to_wallet_id,
                          task_id, event_id, note_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTransaction :one
-- Full-column update (kind is service-frozen; balance recomputes on read).
UPDATE transactions
SET kind           = ?,
    amount_cents   = ?,
    currency       = ?,
    occurred_on    = ?,
    memo           = ?,
    wallet_id      = ?,
    from_wallet_id = ?,
    to_wallet_id   = ?,
    task_id        = ?,
    event_id       = ?,
    note_id        = ?,
    created_at     = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteTransaction :exec
UPDATE transactions
SET deleted_at = sqlc.arg(deleted_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: PurgeTransactionsDeletedBefore :execrows
DELETE
FROM transactions
WHERE deleted_at IS NOT NULL
  AND deleted_at < sqlc.arg(cutoff);

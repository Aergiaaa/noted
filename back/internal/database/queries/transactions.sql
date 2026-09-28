-- name: BalanceSums :one
-- Derived balance: no stored total anywhere (ARCHITECTURE.md D4).
SELECT CAST(COALESCE(SUM(CASE WHEN kind = 'income' THEN amount_cents ELSE 0 END), 0) AS INTEGER)  AS income_cents,
       CAST(COALESCE(SUM(CASE WHEN kind = 'expense' THEN amount_cents ELSE 0 END), 0) AS INTEGER) AS expense_cents
FROM transactions;

-- name: RecentTransactions :many
-- Cursor-free first page (newest first); cursor paging uses the variant below.
SELECT *
FROM transactions
ORDER BY occurred_on DESC, id DESC
LIMIT ?;

-- name: RecentTransactionsBefore :many
-- Keyset page: rows strictly older than (cursor_occurred_on, cursor_id).
SELECT *
FROM transactions
WHERE occurred_on < sqlc.arg(cursor_occurred_on)
   OR (occurred_on = sqlc.arg(cursor_occurred_on) AND id < sqlc.arg(cursor_id))
ORDER BY occurred_on DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetTransaction :one
SELECT *
FROM transactions
WHERE id = ?;

-- name: CreateTransaction :one
INSERT INTO transactions (id, kind, amount_cents, currency, occurred_on, memo, task_id, event_id,
                          note_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTransaction :one
-- Full-column update (balance is recomputed on read, so edits propagate).
UPDATE transactions
SET kind        = ?,
    amount_cents = ?,
    currency     = ?,
    occurred_on  = ?,
    memo         = ?,
    task_id      = ?,
    event_id     = ?,
    note_id      = ?,
    created_at   = ?
WHERE id = ?
RETURNING *;

-- name: DeleteTransaction :exec
DELETE
FROM transactions
WHERE id = ?;

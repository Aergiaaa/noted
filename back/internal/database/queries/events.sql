-- name: ListEventsBetween :many
-- Overlap semantics: an event is in range if it starts before the range
-- ends and ends after the range starts (month/week/day grids).
-- Soft-deleted events are invisible (0004).
SELECT *
FROM events
WHERE starts_at < sqlc.arg(to_time)
  AND ends_at > sqlc.arg(from_time)
  AND deleted_at IS NULL
ORDER BY starts_at;

-- name: GetEvent :one
SELECT *
FROM events
WHERE id = ?
  AND deleted_at IS NULL;

-- name: CreateEvent :one
INSERT INTO events (id, title, starts_at, ends_at, description, note_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateEvent :one
-- Full-column update: the service reads, merges the PATCH, and writes back.
UPDATE events
SET title       = ?,
    starts_at   = ?,
    ends_at     = ?,
    description = ?,
    note_id     = ?,
    updated_at  = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteEvent :exec
-- DELETE /api/events stamps deleted_at; links stay intact so Undo and a
-- later purge keep ON DELETE SET NULL semantics (DATABASE.md).
UPDATE events
SET deleted_at = sqlc.arg(deleted_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: RestoreEvent :one
-- Clears deleted_at; a live row is a successful no-op, a missing id is
-- no rows at all (ErrNotFound).
UPDATE events
SET deleted_at = NULL
WHERE id = ?
RETURNING *;

-- name: ListDeletedEvents :many
-- GET /api/events?trash=1, newest first.
SELECT *
FROM events
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id;

-- name: PurgeEventsDeletedBefore :execrows
-- The only real DELETE: rows past the retention window (30d, DOMAIN.md).
DELETE
FROM events
WHERE deleted_at IS NOT NULL
  AND deleted_at < sqlc.arg(cutoff);

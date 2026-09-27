-- name: ListEventsBetween :many
-- Overlap semantics: an event is in range if it starts before the range
-- ends and ends after the range starts (month/week/day grids).
SELECT *
FROM events
WHERE starts_at < sqlc.arg(to_time) AND ends_at > sqlc.arg(from_time)
ORDER BY starts_at;

-- name: GetEvent :one
SELECT *
FROM events
WHERE id = ?;

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

-- name: DeleteEvent :exec
-- Linked tasks.event_id / transactions.event_id null out (ON DELETE SET NULL).
DELETE
FROM events
WHERE id = ?;

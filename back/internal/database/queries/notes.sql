-- name: ListNotes :many
-- Pinned first, then most recently updated. GET /api/notes?pinned= and the
-- dashboard's pinned card both come through here: nil means "no filter".
SELECT *
FROM notes
WHERE deleted_at IS NULL
  AND (sqlc.narg(pinned) IS NULL OR pinned = sqlc.narg(pinned))
ORDER BY pinned DESC, updated_at DESC;

-- name: GetNote :one
-- Soft-deleted notes are not linkable (events.note_id validation).
SELECT *
FROM notes
WHERE id = ?
  AND deleted_at IS NULL;

-- name: CreateNote :one
INSERT INTO notes (id, title, body_md, pinned, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateNote :one
UPDATE notes
SET title      = ?,
    body_md    = ?,
    pinned     = ?,
    updated_at = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteNote :exec
UPDATE notes
SET deleted_at = sqlc.arg(deleted_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: RestoreNote :one
UPDATE notes
SET deleted_at = NULL
WHERE id = ?
RETURNING *;

-- name: ListDeletedNotes :many
SELECT *
FROM notes
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id;

-- name: PurgeNotesDeletedBefore :execrows
DELETE
FROM notes
WHERE deleted_at IS NOT NULL
  AND deleted_at < sqlc.arg(cutoff);

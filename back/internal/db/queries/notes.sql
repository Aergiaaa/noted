-- name: ListNotes :many
-- Pinned first, then most recently updated. GET /api/notes?pinned= and the
-- dashboard's pinned card both come through here: nil means "no filter".
SELECT *
FROM notes
WHERE sqlc.narg(pinned) IS NULL OR pinned = sqlc.narg(pinned)
ORDER BY pinned DESC, updated_at DESC;

-- name: GetNote :one
SELECT *
FROM notes
WHERE id = ?;

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

-- name: DeleteNote :exec
-- Linked events.task/note links null out (ON DELETE SET NULL).
DELETE
FROM notes
WHERE id = ?;

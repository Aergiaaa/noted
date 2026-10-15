-- name: ListTasks :many
-- GET /api/tasks?status=&due= - nil argument means "no filter".
SELECT *
FROM tasks
WHERE deleted_at IS NULL
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(due) IS NULL OR due_date = sqlc.narg(due))
ORDER BY (due_date IS NULL), due_date, created_at;

-- name: ListOpenTasksDue :many
-- Dashboard "due today": open tasks on the given calendar day.
SELECT *
FROM tasks
WHERE status = 'open'
  AND due_date = ?
  AND deleted_at IS NULL
ORDER BY created_at;

-- name: ListOpenTasksDueBefore :many
-- Dashboard "overdue": open tasks due before the given calendar day.
SELECT *
FROM tasks
WHERE status = 'open'
  AND due_date < ?
  AND deleted_at IS NULL
ORDER BY due_date;

-- name: GetTask :one
-- Soft-deleted tasks are not linkable (transactions.task_id validation).
SELECT *
FROM tasks
WHERE id = ?
  AND deleted_at IS NULL;

-- name: CreateTask :one
INSERT INTO tasks (id, title, status, due_date, event_id, note_id, created_at, updated_at, completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTask :one
-- Full-column update; completed_at NULL reopens the task.
UPDATE tasks
SET title        = ?,
    status       = ?,
    due_date     = ?,
    event_id     = ?,
    note_id      = ?,
    updated_at   = ?,
    completed_at = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteTask :exec
UPDATE tasks
SET deleted_at = sqlc.arg(deleted_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: RestoreTask :one
UPDATE tasks
SET deleted_at = NULL
WHERE id = ?
RETURNING *;

-- name: ListDeletedTasks :many
SELECT *
FROM tasks
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id;

-- name: PurgeTasksDeletedBefore :execrows
DELETE
FROM tasks
WHERE deleted_at IS NOT NULL
  AND deleted_at < sqlc.arg(cutoff);

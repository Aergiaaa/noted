-- name: ListTasks :many
-- GET /api/tasks?status=&due= - nil argument means "no filter".
SELECT *
FROM tasks
WHERE (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(due) IS NULL OR due_date = sqlc.narg(due))
ORDER BY (due_date IS NULL), due_date, created_at;

-- name: ListOpenTasksDue :many
-- Dashboard "due today": open tasks on the given calendar day.
SELECT *
FROM tasks
WHERE status = 'open' AND due_date = ?
ORDER BY created_at;

-- name: ListOpenTasksDueBefore :many
-- Dashboard "overdue": open tasks due before the given calendar day.
SELECT *
FROM tasks
WHERE status = 'open' AND due_date < ?
ORDER BY due_date;

-- name: GetTask :one
SELECT *
FROM tasks
WHERE id = ?;

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

-- name: DeleteTask :exec
DELETE
FROM tasks
WHERE id = ?;

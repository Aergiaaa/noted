package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"noted/internal/database/generated"
)

// Task status values (DOMAIN.md); the DB CHECK mirrors them.
const (
	STATUS_OPEN = "open"
	STATUS_DONE = "done"
)

// TaskInput is a fully-resolved task: a create payload, or a PATCH row
// after the service merged it over the stored one. Status/CompletedAt
// carry the merged-update path only — Create always starts a task open
// (API.md POST has no status field). Optional fields use "" for "no
// value" (no due date / no link / not completed); CompletedAt is set only
// on the merged-update path.
type TaskInput struct {
	Title       string
	Status      string
	DueDate     string
	EventID     string
	NoteID      string
	CompletedAt string
}

// TaskPatch: a nil field keeps the stored value; "" clears due_date and
// links (the form PATCHes every field, "" = unlink — nulled at write).
type TaskPatch struct {
	Title   *string
	DueDate *string
	EventID *string
	NoteID  *string
	Status  *string
}

// Task is the domain view of a row; timestamps stay the stored UTC
// strings (already the wire format, like Event).
type Task struct {
	ID          string
	Title       string
	Status      string
	DueDate     *string
	EventID     *string
	NoteID      *string
	CreatedAt   string
	UpdatedAt   string
	CompletedAt *string
}

type TaskService struct {
	q *generated.Queries
}

func NewTaskService(q *generated.Queries) *TaskService {
	return &TaskService{q: q}
}

// List: nil status/due means no filter — both arrive already validated
// (status enum, due YYYY-MM-DD) from the handler's query parsing, and
// ListTasks applies them as narg filters ordered by due date.
func (s *TaskService) List(ctx context.Context, status, due *string) ([]Task, error) {
	rows, err := s.q.ListTasks(ctx, generated.ListTasksParams{
		Status: derefFilter(status),
		Due:    derefFilter(due),
	})
	if err != nil {
		return nil, err
	}

	tasks := make([]Task, len(rows))
	for i, row := range rows {
		tasks[i] = fromTaskRow(row)
	}

	return tasks, nil
}

func (s *TaskService) Create(ctx context.Context, in TaskInput) (Task, error) {
	in.Title = sanitizeLine(in.Title)
	in.Status = STATUS_OPEN
	if err := s.validate(ctx, in, linkScope{event: true, note: true}); err != nil {
		return Task{}, err
	}

	now := formatStored(time.Now())
	row, err := s.q.CreateTask(ctx, generated.CreateTaskParams{
		ID:        uuid.NewString(),
		Title:     in.Title,
		Status:    in.Status,
		DueDate:   nullIfEmpty(in.DueDate),
		EventID:   nullIfEmpty(in.EventID),
		NoteID:    nullIfEmpty(in.NoteID),
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return Task{}, err
	}

	return fromTaskRow(row), nil
}

// Update reads the row, merges the patch over it, re-validates the whole
// (UpdateTask wants every column anyway), and bumps updated_at.
func (s *TaskService) Update(ctx context.Context, id string, patch TaskPatch) (Task, error) {
	row, err := s.q.GetTask(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}

	merged := mergeTaskPatch(row, patch)
	merged.Title = sanitizeLine(merged.Title)
	if err := s.validate(ctx, merged, linkScope{
		event: changedLink(patch.EventID, row.EventID),
		note:  changedLink(patch.NoteID, row.NoteID),
	}); err != nil {
		return Task{}, err
	}

	updated, err := s.q.UpdateTask(ctx, generated.UpdateTaskParams{
		Title:       merged.Title,
		Status:      merged.Status,
		DueDate:     nullIfEmpty(merged.DueDate),
		EventID:     nullIfEmpty(merged.EventID),
		NoteID:      nullIfEmpty(merged.NoteID),
		UpdatedAt:   formatStored(time.Now()),
		ID:          id,
		CompletedAt: nullIfEmpty(merged.CompletedAt),
	})
	if err != nil {
		return Task{}, err
	}

	return fromTaskRow(updated), nil
}

// Delete is idempotent: it stamps deleted_at (204) and opportunistically
// purges rows past retention; boot retries. Purge is the only real DELETE,
// and ON DELETE SET NULL frees inbound transaction links (DOMAIN.md).
func (s *TaskService) Delete(ctx context.Context, id string) error {
	now := formatStored(time.Now())
	if err := s.q.SoftDeleteTask(ctx, generated.SoftDeleteTaskParams{
		DeletedAt: &now,
		ID:        id,
	}); err != nil {
		return err
	}

	_, purgeErr := s.Purge(ctx, time.Now())
	logPurge("tasks", purgeErr)

	return nil
}

// Purge hard-deletes rows soft-deleted before the retention cutoff and
// reports how many went. Boot calls it for every domain table.
func (s *TaskService) Purge(ctx context.Context, now time.Time) (int64, error) {
	cutoff := formatStored(now.Add(-SOFT_DELETE_RETENTION))
	return s.q.PurgeTasksDeletedBefore(ctx, &cutoff)
}

func (s *TaskService) validate(ctx context.Context, in TaskInput, scope linkScope) error {
	fields := map[string]string{}
	switch {
	case in.Title == "":
		fields["title"] = "required"
	case len([]rune(in.Title)) > TITLE_MAX_CHAR:
		fields["title"] = titleTooLong()
	}

	if in.Status != STATUS_OPEN && in.Status != STATUS_DONE {
		fields["status"] = "must be open or done"
	}

	if scope.event {
		if err := s.validateEventLink(ctx, fields, in.EventID); err != nil {
			return err
		}
	}
	if scope.note {
		if err := s.validateNoteLink(ctx, fields, in.NoteID); err != nil {
			return err
		}
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	return nil
}

// validateEventLink records "unknown event" for a non-empty link id; a
// storage failure aborts validation with the raw error (writeServiceError
// maps it to the generic 500).
func (s *TaskService) validateEventLink(ctx context.Context, fields map[string]string, eventID string) error {
	if eventID == "" {
		return nil
	}

	_, err := s.q.GetEvent(ctx, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		fields["event_id"] = "unknown event"
		return nil
	}
	if err != nil {
		return err
	}

	return nil
}

func (s *TaskService) validateNoteLink(ctx context.Context, fields map[string]string, noteID string) error {
	if noteID == "" {
		return nil
	}

	_, err := s.q.GetNote(ctx, noteID)
	if errors.Is(err, sql.ErrNoRows) {
		fields["note_id"] = "unknown note"
		return nil
	}
	if err != nil {
		return err
	}

	return nil
}

// mergeTaskPatch overlays a partial patch on the stored row; a status
// change carries the completed_at rule with it.
func mergeTaskPatch(row generated.Task, patch TaskPatch) TaskInput {
	in := TaskInput{
		Title:       row.Title,
		Status:      row.Status,
		DueDate:     derefString(row.DueDate),
		EventID:     derefString(row.EventID),
		NoteID:      derefString(row.NoteID),
		CompletedAt: derefString(row.CompletedAt),
	}

	if patch.Title != nil {
		in.Title = *patch.Title
	}
	if patch.DueDate != nil {
		in.DueDate = *patch.DueDate
	}
	if patch.EventID != nil {
		in.EventID = *patch.EventID
	}
	if patch.NoteID != nil {
		in.NoteID = *patch.NoteID
	}
	if patch.Status != nil {
		in.Status = *patch.Status
		in.CompletedAt = completedAtFor(*patch.Status)
	}
	return in
}

// completedAtFor: done stamps now, open clears ("" = no completed_at).
// A bogus status lands here as "" first and is rejected by validate
// before any write, so the stamping stays unreachable for values outside
// the STATUS enum.
func completedAtFor(status string) string {
	if status == STATUS_DONE {
		return formatStored(time.Now())
	}
	return ""
}

// derefFilter: sqlc narg params bind nil (no filter) or the plain value.
func derefFilter(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func fromTaskRow(row generated.Task) Task {
	return Task{
		ID:          row.ID,
		Title:       row.Title,
		Status:      row.Status,
		DueDate:     row.DueDate,
		EventID:     row.EventID,
		NoteID:      row.NoteID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		CompletedAt: row.CompletedAt,
	}
}

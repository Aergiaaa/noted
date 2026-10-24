package service

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noted/internal/database"
	"noted/internal/database/generated"
)

func newTestTaskService(t *testing.T) (*TaskService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewTaskService(generated.New(db)), db
}

func seedEventForTask(t *testing.T, db *sql.DB) string {
	t.Helper()
	created, err := NewEventService(generated.New(db)).Create(context.Background(), EventInput{
		Title:    "Anchor",
		StartsAt: mustTime(t, "2026-10-14T09:00:00Z"),
		EndsAt:   mustTime(t, "2026-10-14T10:00:00Z"),
	})
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	return created.ID
}

func seedNoteForTask(t *testing.T, db *sql.DB) string {
	t.Helper()
	created, err := NewNoteService(generated.New(db)).Create(context.Background(), NoteInput{Title: "Context"})
	if err != nil {
		t.Fatalf("seed note: %v", err)
	}
	return created.ID
}

func seedTask(t *testing.T, svc *TaskService, in TaskInput) Task {
	t.Helper()
	created, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("seed task %q: %v", in.Title, err)
	}
	return created
}

func TestTaskService_create_startsOpenWithoutCompletion(t *testing.T) {
	svc, _ := newTestTaskService(t)

	created, err := svc.Create(context.Background(), TaskInput{
		Title:       "  Buy milk  ",
		Status:      STATUS_DONE,
		CompletedAt: ptr("2026-10-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("id/timestamps empty: %+v", created)
	}
	if created.Title != "Buy milk" {
		t.Errorf("Title = %q, want trimmed", created.Title)
	}
	if created.Status != STATUS_OPEN {
		t.Errorf("Status = %q, want a create to always start %q", created.Status, STATUS_OPEN)
	}
	if created.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil on create", created.CompletedAt)
	}
	if created.DueDate != nil || created.EventID != nil || created.NoteID != nil {
		t.Errorf("due/links = %v/%v/%v, want nil defaults", created.DueDate, created.EventID, created.NoteID)
	}
}

func TestTaskService_create_emptyTitle_400(t *testing.T) {
	svc, _ := newTestTaskService(t)

	_, err := svc.Create(context.Background(), TaskInput{Title: "   "})
	if got := fieldsOf(t, err)["title"]; got != "required" {
		t.Fatalf("title field = %q, want required", got)
	}
}

func TestTaskService_create_titleTooLong_400(t *testing.T) {
	svc, _ := newTestTaskService(t)

	_, err := svc.Create(context.Background(), TaskInput{Title: strings.Repeat("x", 201)})
	if got := fieldsOf(t, err)["title"]; got != "must be at most 200 characters" {
		t.Fatalf("title field = %q", got)
	}
}

func TestTaskService_create_persistsDueAndLinks(t *testing.T) {
	svc, db := newTestTaskService(t)
	eventID := seedEventForTask(t, db)
	noteID := seedNoteForTask(t, db)

	created := seedTask(t, svc, TaskInput{
		Title:   "Buy keyboard",
		DueDate: ptr("2026-10-20"),
		EventID: &eventID,
		NoteID:  &noteID,
	})
	if created.DueDate == nil || *created.DueDate != "2026-10-20" {
		t.Errorf("DueDate = %v, want 2026-10-20", created.DueDate)
	}
	if created.EventID == nil || *created.EventID != eventID {
		t.Errorf("EventID = %v, want %q", created.EventID, eventID)
	}
	if created.NoteID == nil || *created.NoteID != noteID {
		t.Errorf("NoteID = %v, want %q", created.NoteID, noteID)
	}
}

func TestTaskService_create_unknownEvent_400(t *testing.T) {
	svc, _ := newTestTaskService(t)

	_, err := svc.Create(context.Background(), TaskInput{Title: "X", EventID: ptr("nope")})
	if got := fieldsOf(t, err)["event_id"]; got != "unknown event" {
		t.Fatalf("event_id field = %q, want unknown event", got)
	}
}

func TestTaskService_create_unknownNote_400(t *testing.T) {
	svc, _ := newTestTaskService(t)

	_, err := svc.Create(context.Background(), TaskInput{Title: "X", NoteID: ptr("nope")})
	if got := fieldsOf(t, err)["note_id"]; got != "unknown note" {
		t.Fatalf("note_id field = %q, want unknown note", got)
	}
}

func TestTaskService_create_stripsControlCharacters(t *testing.T) {
	svc, _ := newTestTaskService(t)

	created, err := svc.Create(context.Background(), TaskInput{Title: "Den\x00t\tist "})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Title != "Dentist" {
		t.Errorf("Title = %q, want controls stripped", created.Title)
	}
}

func TestTaskService_create_controlOnlyTitle_400(t *testing.T) {
	svc, _ := newTestTaskService(t)

	_, err := svc.Create(context.Background(), TaskInput{Title: "\x00\x07\t\n"})
	if got := fieldsOf(t, err)["title"]; got != "required" {
		t.Fatalf("title field = %q, want required", got)
	}
}

func TestTaskService_create_eventTableGone_internalError(t *testing.T) {
	svc, db := newTestTaskService(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}

	_, err := svc.Create(context.Background(), TaskInput{Title: "X", EventID: ptr("e1")})
	if err == nil {
		t.Fatal("Create succeeded, want internal error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = %v, want internal error not validation", err)
	}
}

func TestTaskService_create_noteTableGone_internalError(t *testing.T) {
	svc, db := newTestTaskService(t)
	if _, err := db.Exec(`DROP TABLE notes`); err != nil {
		t.Fatalf("drop notes: %v", err)
	}

	_, err := svc.Create(context.Background(), TaskInput{Title: "X", NoteID: ptr("n1")})
	if err == nil {
		t.Fatal("Create succeeded, want internal error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = %v, want internal error not validation", err)
	}
}

func TestTaskService_create_taskTableGone_internalError(t *testing.T) {
	svc, db := newTestTaskService(t)
	if _, err := db.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("drop tasks: %v", err)
	}

	_, err := svc.Create(context.Background(), TaskInput{Title: "X"})
	if err == nil {
		t.Fatal("Create succeeded, want internal error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = %v, want internal error not validation", err)
	}
}

func TestTaskService_list_ordersByDueDateNullsLast(t *testing.T) {
	svc, _ := newTestTaskService(t)
	seedTask(t, svc, TaskInput{Title: "Later", DueDate: ptr("2026-10-10")})
	seedTask(t, svc, TaskInput{Title: "Sooner", DueDate: ptr("2026-10-05")})
	seedTask(t, svc, TaskInput{Title: "NoDue"})

	tasks, err := svc.List(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"Sooner", "Later", "NoDue"}
	if len(tasks) != len(want) {
		t.Fatalf("List len = %d, want %d", len(tasks), len(want))
	}
	for i, title := range want {
		if tasks[i].Title != title {
			t.Errorf("List[%d] = %q, want %q", i, tasks[i].Title, title)
		}
	}
}

func TestTaskService_list_statusFilter(t *testing.T) {
	svc, _ := newTestTaskService(t)
	seedTask(t, svc, TaskInput{Title: "Open one"})
	done := seedTask(t, svc, TaskInput{Title: "Done one"})
	if _, err := svc.Update(context.Background(), done.ID, TaskPatch{Status: ptr(STATUS_DONE)}); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	open, err := svc.List(context.Background(), ptr(STATUS_OPEN), nil)
	if err != nil {
		t.Fatalf("List open: %v", err)
	}
	if len(open) != 1 || open[0].Title != "Open one" {
		t.Fatalf("List(open) = %+v, want the open task", open)
	}

	onlyDone, err := svc.List(context.Background(), ptr(STATUS_DONE), nil)
	if err != nil {
		t.Fatalf("List done: %v", err)
	}
	if len(onlyDone) != 1 || onlyDone[0].Title != "Done one" {
		t.Fatalf("List(done) = %+v, want the done task", onlyDone)
	}
}

func TestTaskService_list_dueFilter(t *testing.T) {
	svc, _ := newTestTaskService(t)
	seedTask(t, svc, TaskInput{Title: "Target", DueDate: ptr("2026-10-05")})
	seedTask(t, svc, TaskInput{Title: "Other", DueDate: ptr("2026-10-10")})
	seedTask(t, svc, TaskInput{Title: "NoDue"})

	tasks, err := svc.List(context.Background(), nil, ptr("2026-10-05"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Target" {
		t.Fatalf("List(due) = %+v, want only the 2026-10-05 task", tasks)
	}
}

func TestTaskService_list_statusAndDueCombined(t *testing.T) {
	svc, _ := newTestTaskService(t)
	seedTask(t, svc, TaskInput{Title: "Open05", DueDate: ptr("2026-10-05")})
	done := seedTask(t, svc, TaskInput{Title: "Done05", DueDate: ptr("2026-10-05")})
	if _, err := svc.Update(context.Background(), done.ID, TaskPatch{Status: ptr(STATUS_DONE)}); err != nil {
		t.Fatalf("mark done: %v", err)
	}
	seedTask(t, svc, TaskInput{Title: "Open10", DueDate: ptr("2026-10-10")})

	tasks, err := svc.List(context.Background(), ptr(STATUS_OPEN), ptr("2026-10-05"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Open05" {
		t.Fatalf("List(open+due) = %+v, want only Open05", tasks)
	}
}

func TestTaskService_list_taskTableGone_internalError(t *testing.T) {
	svc, db := newTestTaskService(t)
	if _, err := db.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("drop tasks: %v", err)
	}

	if _, err := svc.List(context.Background(), nil, nil); err == nil {
		t.Fatal("List succeeded, want internal error")
	}
}

func TestTaskService_update_stripsControlCharacters(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Plain"})
	title := "Den\x00tist\x1b"

	updated, err := svc.Update(context.Background(), task.ID, TaskPatch{Title: &title})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Dentist" {
		t.Errorf("Title = %q, want controls stripped", updated.Title)
	}
}

func TestTaskService_update_mergesPartial(t *testing.T) {
	svc, db := newTestTaskService(t)
	eventID := seedEventForTask(t, db)
	noteID := seedNoteForTask(t, db)
	task := seedTask(t, svc, TaskInput{
		Title:   "Old",
		DueDate: ptr("2026-10-10"),
		EventID: &eventID,
		NoteID:  &noteID,
	})

	updated, err := svc.Update(context.Background(), task.ID, TaskPatch{Title: ptr("New")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "New" {
		t.Errorf("Title = %q, want patched", updated.Title)
	}
	if updated.DueDate == nil || *updated.DueDate != "2026-10-10" {
		t.Errorf("DueDate = %v, want kept", updated.DueDate)
	}
	if updated.EventID == nil || *updated.EventID != eventID {
		t.Errorf("EventID = %v, want kept", updated.EventID)
	}
	if updated.NoteID == nil || *updated.NoteID != noteID {
		t.Errorf("NoteID = %v, want kept", updated.NoteID)
	}
	if updated.Status != STATUS_OPEN {
		t.Errorf("Status = %q, want kept open", updated.Status)
	}
}

func TestTaskService_update_setsDueAndLinks(t *testing.T) {
	svc, db := newTestTaskService(t)
	eventID := seedEventForTask(t, db)
	task := seedTask(t, svc, TaskInput{Title: "Bare"})

	updated, err := svc.Update(context.Background(), task.ID, TaskPatch{
		DueDate: ptr("2026-10-20"),
		EventID: &eventID,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DueDate == nil || *updated.DueDate != "2026-10-20" {
		t.Errorf("DueDate = %v, want 2026-10-20", updated.DueDate)
	}
	if updated.EventID == nil || *updated.EventID != eventID {
		t.Errorf("EventID = %v, want %q", updated.EventID, eventID)
	}
}

func TestTaskService_update_emptyStringsClearStoredLinks(t *testing.T) {
	svc, db := newTestTaskService(t)
	eventID := seedEventForTask(t, db)
	noteID := seedNoteForTask(t, db)
	task := seedTask(t, svc, TaskInput{
		Title:   "Linked",
		DueDate: ptr("2026-10-20"),
		EventID: &eventID,
		NoteID:  &noteID,
	})

	updated, err := svc.Update(context.Background(), task.ID, TaskPatch{
		DueDate: ptr(""),
		EventID: ptr(""),
		NoteID:  ptr(""),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DueDate != nil || updated.EventID != nil || updated.NoteID != nil {
		t.Fatalf("due/links = %v/%v/%v, want all cleared", updated.DueDate, updated.EventID, updated.NoteID)
	}
}

func TestTaskService_update_statusDone_setsCompletedAt(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Finish"})

	updated, err := svc.Update(context.Background(), task.ID, TaskPatch{Status: ptr(STATUS_DONE)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Status != STATUS_DONE {
		t.Errorf("Status = %q, want done", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Fatal("CompletedAt = nil, want set on done")
	}
	if _, err := time.Parse(time.RFC3339, *updated.CompletedAt); err != nil {
		t.Errorf("CompletedAt = %q, want RFC3339", *updated.CompletedAt)
	}
}

func TestTaskService_update_statusOpen_clearsCompletedAt(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Reopen me"})
	if _, err := svc.Update(context.Background(), task.ID, TaskPatch{Status: ptr(STATUS_DONE)}); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	reopened, err := svc.Update(context.Background(), task.ID, TaskPatch{Status: ptr(STATUS_OPEN)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if reopened.Status != STATUS_OPEN {
		t.Errorf("Status = %q, want open", reopened.Status)
	}
	if reopened.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want cleared on open", reopened.CompletedAt)
	}
}

func TestTaskService_update_withoutStatusKeepsCompletedAt(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Done task"})
	done, err := svc.Update(context.Background(), task.ID, TaskPatch{Status: ptr(STATUS_DONE)})
	if err != nil {
		t.Fatalf("mark done: %v", err)
	}

	renamed, err := svc.Update(context.Background(), task.ID, TaskPatch{Title: ptr("Renamed")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if renamed.Status != STATUS_DONE {
		t.Errorf("Status = %q, want kept done", renamed.Status)
	}
	if renamed.CompletedAt == nil || *renamed.CompletedAt != *done.CompletedAt {
		t.Errorf("CompletedAt = %v, want kept %v", renamed.CompletedAt, done.CompletedAt)
	}
}

func TestTaskService_update_invalidStatus_400(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Seed"})

	_, err := svc.Update(context.Background(), task.ID, TaskPatch{Status: ptr("maybe")})
	if got := fieldsOf(t, err)["status"]; got != "must be open or done" {
		t.Fatalf("status field = %q, want must be open or done", got)
	}
}

func TestTaskService_update_missing_404(t *testing.T) {
	svc, _ := newTestTaskService(t)

	_, err := svc.Update(context.Background(), "ghost", TaskPatch{Title: ptr("X")})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestTaskService_update_unknownEvent_400(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Seed"})

	_, err := svc.Update(context.Background(), task.ID, TaskPatch{EventID: ptr("nope")})
	if got := fieldsOf(t, err)["event_id"]; got != "unknown event" {
		t.Fatalf("event_id field = %q, want unknown event", got)
	}
}

func TestTaskService_update_unknownNote_400(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Seed"})

	_, err := svc.Update(context.Background(), task.ID, TaskPatch{NoteID: ptr("nope")})
	if got := fieldsOf(t, err)["note_id"]; got != "unknown note" {
		t.Fatalf("note_id field = %q, want unknown note", got)
	}
}

func TestTaskService_update_keepsStoredTrashedEventLink(t *testing.T) {
	svc, db := newTestTaskService(t)
	eventID := seedEventForTask(t, db)
	task := seedTask(t, svc, TaskInput{Title: "Buy keyboard", EventID: &eventID})
	if _, err := db.Exec(
		`UPDATE events SET deleted_at = '2020-01-01T00:00:00Z' WHERE id = ?`, eventID); err != nil {
		t.Fatalf("trash event: %v", err)
	}

	// TaskForm omits the link keys, so the stored trashed event is kept
	// (API.md absent = keep) and the title edit must go through.
	updated, err := svc.Update(context.Background(), task.ID, TaskPatch{Title: ptr("Buy keyboard tomorrow")})
	if err != nil {
		t.Fatalf("Update title with stored trashed event link: %v", err)
	}
	if updated.EventID == nil || *updated.EventID != eventID {
		t.Fatalf("event_id = %v, want kept %s", updated.EventID, eventID)
	}
}

func TestTaskService_update_replaceWithUnknownEvent_400(t *testing.T) {
	svc, db := newTestTaskService(t)
	eventID := seedEventForTask(t, db)
	task := seedTask(t, svc, TaskInput{Title: "Buy keyboard", EventID: &eventID})

	_, err := svc.Update(context.Background(), task.ID, TaskPatch{EventID: ptr("e_missing")})
	if got := fieldsOf(t, err)["event_id"]; got != "unknown event" {
		t.Fatalf("event_id field = %q, want unknown event", got)
	}
}

func TestTaskService_update_emptyTitle_400(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Seed"})

	_, err := svc.Update(context.Background(), task.ID, TaskPatch{Title: ptr("  ")})
	if got := fieldsOf(t, err)["title"]; got != "required" {
		t.Fatalf("title field = %q, want required", got)
	}
}

func TestTaskService_update_taskTableGone_internalError(t *testing.T) {
	svc, db := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Seed"})
	if _, err := db.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("drop tasks: %v", err)
	}

	if _, err := svc.Update(context.Background(), task.ID, TaskPatch{Title: ptr("X")}); err == nil {
		t.Fatal("Update succeeded, want internal error")
	}
}

func TestTaskService_update_writeConflict_internalError(t *testing.T) {
	svc, db := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Seed"})
	if _, err := db.Exec(
		`CREATE TRIGGER fail_update BEFORE UPDATE ON tasks
		 BEGIN SELECT RAISE(ABORT, 'nope'); END`,
	); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	if _, err := svc.Update(context.Background(), task.ID, TaskPatch{Title: ptr("X")}); err == nil {
		t.Fatal("Update succeeded, want write error")
	}
}

func TestTaskService_delete_softDeletedLeavesList(t *testing.T) {
	svc, _ := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Doomed"})

	if err := svc.Delete(context.Background(), task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	tasks, err := svc.List(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("List after delete = %+v, want empty", tasks)
	}

	if _, err := svc.Update(context.Background(), task.ID, TaskPatch{Title: ptr("X")}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update after delete err = %v, want ErrNotFound", err)
	}
}

func TestTaskService_delete_missing_isIdempotent(t *testing.T) {
	svc, _ := newTestTaskService(t)
	if err := svc.Delete(context.Background(), "ghost"); err != nil {
		t.Fatalf("Delete ghost: %v", err)
	}
}

func TestTaskService_delete_taskTableGone_internalError(t *testing.T) {
	svc, db := newTestTaskService(t)
	if _, err := db.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("drop tasks: %v", err)
	}

	if err := svc.Delete(context.Background(), "any"); err == nil {
		t.Fatal("Delete succeeded, want internal error")
	}
}

func TestTaskService_delete_purgesExpiredRows(t *testing.T) {
	svc, db := newTestTaskService(t)
	old := seedTask(t, svc, TaskInput{Title: "Old"})
	if _, err := db.Exec(
		`UPDATE tasks SET deleted_at = '2020-01-01T00:00:00Z' WHERE id = ?`, old.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	fresh := seedTask(t, svc, TaskInput{Title: "Fresh"})

	if err := svc.Delete(context.Background(), fresh.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var oldRows, freshRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = ?`, old.ID).Scan(&oldRows); err != nil {
		t.Fatalf("count old: %v", err)
	}
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE id = ? AND deleted_at IS NOT NULL`, fresh.ID,
	).Scan(&freshRows); err != nil {
		t.Fatalf("count fresh: %v", err)
	}
	if oldRows != 0 || freshRows != 1 {
		t.Fatalf("rows old/fresh = %d/%d, want 0 (purged) / 1 (soft-deleted)", oldRows, freshRows)
	}
}

func TestTaskService_purge_removesOnlyPastCutoff_nullsInboundTransactionLink(t *testing.T) {
	svc, db := newTestTaskService(t)
	task := seedTask(t, svc, TaskInput{Title: "Linked"})
	if _, err := db.Exec(
		`INSERT INTO transactions (id, kind, amount_cents, currency, occurred_on, task_id, created_at)
		 VALUES ('x1', 'expense', 5000, 'USD', '2026-09-20', ?, '2026-09-20T00:00:00Z')`,
		task.ID,
	); err != nil {
		t.Fatalf("seed transaction: %v", err)
	}

	if err := svc.Delete(context.Background(), task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE tasks SET deleted_at = '2020-01-01T00:00:00Z' WHERE id = ?`, task.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	n, err := svc.Purge(context.Background(), mustTime(t, "2026-10-15T00:00:00Z"))
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 1 {
		t.Fatalf("Purge rows = %d, want 1", n)
	}

	var taskRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = ?`, task.ID).Scan(&taskRows); err != nil {
		t.Fatalf("count task: %v", err)
	}
	if taskRows != 0 {
		t.Fatalf("task rows = %d, want 0 (hard-deleted)", taskRows)
	}

	var taskID sql.NullString
	if err := db.QueryRow(`SELECT task_id FROM transactions WHERE id = 'x1'`).Scan(&taskID); err != nil {
		t.Fatalf("read transaction: %v", err)
	}
	if taskID.Valid {
		t.Fatalf("transaction.task_id = %q, want NULL after purge", taskID.String)
	}
}

func TestTaskService_purge_zeroWhenNothingDue(t *testing.T) {
	svc, _ := newTestTaskService(t)
	n, err := svc.Purge(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 0 {
		t.Fatalf("Purge rows = %d, want 0", n)
	}
}

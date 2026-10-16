package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"noted/internal/database"
	"noted/internal/database/generated"
	"noted/internal/service"

	"github.com/go-chi/chi/v5"
)

// newTestTasksHandler mirrors router.go's route table for the task
// endpoints so {id} params resolve as they do in production.
func newTestTasksHandler(t *testing.T) (*Handler, http.Handler, *service.TaskService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "handler-tasks.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := service.NewTaskService(generated.New(db))
	h := New(nil, nil, nil, svc, false)
	r := chi.NewRouter()
	r.Get("/api/tasks", h.ListTasks)
	r.Post("/api/tasks", h.CreateTask)
	r.Patch("/api/tasks/{id}", h.UpdateTask)
	r.Delete("/api/tasks/{id}", h.DeleteTask)
	return h, r, svc, db
}

func decodeTask(t *testing.T, rec *httptest.ResponseRecorder) taskJSON {
	t.Helper()
	var task taskJSON
	if err := json.NewDecoder(rec.Body).Decode(&task); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	return task
}

func createTaskViaAPI(t *testing.T, router http.Handler, body string) taskJSON {
	t.Helper()
	rec := httpRequest(t, router, http.MethodPost, "/api/tasks", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed status = %d, body %s", rec.Code, rec.Body.String())
	}
	return decodeTask(t, rec)
}

func tasksListData(t *testing.T, rec *httptest.ResponseRecorder) []taskJSON {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []taskJSON `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Data
}

func seedLinkedEvent(t *testing.T, db *sql.DB) string {
	t.Helper()
	created, err := service.NewEventService(generated.New(db)).Create(context.Background(), service.EventInput{
		Title:    "Anchor",
		StartsAt: rfc(t, "2026-10-14T09:00:00Z"),
		EndsAt:   rfc(t, "2026-10-14T10:00:00Z"),
	})
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	return created.ID
}

func seedLinkedNote(t *testing.T, db *sql.DB) string {
	t.Helper()
	created, err := service.NewNoteService(generated.New(db)).Create(context.Background(), service.NoteInput{Title: "Context"})
	if err != nil {
		t.Fatalf("seed note: %v", err)
	}
	return created.ID
}

func TestListTasks_ok_dueDateOrder(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	createTaskViaAPI(t, router, `{"title":"Later","due_date":"2026-10-10"}`)
	createTaskViaAPI(t, router, `{"title":"Sooner","due_date":"2026-10-05"}`)

	data := tasksListData(t, httpRequest(t, router, http.MethodGet, "/api/tasks", ""))
	if len(data) != 2 || data[0].Title != "Sooner" || data[1].Title != "Later" {
		t.Fatalf("data = %+v, want [Sooner Later]", data)
	}
}

func TestListTasks_statusFilters(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	createTaskViaAPI(t, router, `{"title":"Open one"}`)
	done := createTaskViaAPI(t, router, `{"title":"Done one"}`)
	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/"+done.ID, `{"status":"done"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("mark done status = %d, body %s", rec.Code, rec.Body.String())
	}

	open := tasksListData(t, httpRequest(t, router, http.MethodGet, "/api/tasks?status=open", ""))
	if len(open) != 1 || open[0].Title != "Open one" {
		t.Fatalf("?status=open = %+v, want the open task", open)
	}

	onlyDone := tasksListData(t, httpRequest(t, router, http.MethodGet, "/api/tasks?status=done", ""))
	if len(onlyDone) != 1 || onlyDone[0].Title != "Done one" {
		t.Fatalf("?status=done = %+v, want the done task", onlyDone)
	}

	all := tasksListData(t, httpRequest(t, router, http.MethodGet, "/api/tasks?status=all", ""))
	if len(all) != 2 {
		t.Fatalf("?status=all = %+v, want both tasks", all)
	}
}

func TestListTasks_dueFilter(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	createTaskViaAPI(t, router, `{"title":"Target","due_date":"2026-10-05"}`)
	createTaskViaAPI(t, router, `{"title":"Other","due_date":"2026-10-10"}`)

	data := tasksListData(t, httpRequest(t, router, http.MethodGet, "/api/tasks?due=2026-10-05", ""))
	if len(data) != 1 || data[0].Title != "Target" {
		t.Fatalf("?due=2026-10-05 = %+v, want only the target task", data)
	}
}

func TestListTasks_invalidStatus_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodGet, "/api/tasks?status=maybe", "")

	status, code, fields := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
	if fields["status"] != "must be open, done, or all" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestListTasks_invalidDue_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodGet, "/api/tasks?due=15-10-2026", "")

	_, _, fields := decodeErr(t, rec)
	if fields["due"] != "must be YYYY-MM-DD" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestListTasks_internal_500Envelope(t *testing.T) {
	_, router, _, db := newTestTasksHandler(t)
	if _, err := db.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("drop tasks: %v", err)
	}

	rec := httpRequest(t, router, http.MethodGet, "/api/tasks", "")
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestCreateTask_ok_201_withDueAndLinks(t *testing.T) {
	_, router, _, db := newTestTasksHandler(t)
	eventID := seedLinkedEvent(t, db)
	noteID := seedLinkedNote(t, db)

	rec := httpRequest(t, router, http.MethodPost, "/api/tasks",
		`{"title":"  Buy keyboard  ","due_date":"2026-09-20","event_id":"`+eventID+`","note_id":"`+noteID+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	task := decodeTask(t, rec)
	if task.ID == "" || task.CreatedAt == "" || task.UpdatedAt == "" {
		t.Fatalf("wire shape incomplete: %+v", task)
	}
	if task.Title != "Buy keyboard" {
		t.Errorf("Title = %q, want trimmed", task.Title)
	}
	if task.Status != "open" || task.CompletedAt != nil {
		t.Errorf("status/completed = %q/%v, want open/nil", task.Status, task.CompletedAt)
	}
	if task.DueDate == nil || *task.DueDate != "2026-09-20" {
		t.Errorf("DueDate = %v, want 2026-09-20", task.DueDate)
	}
	if task.EventID == nil || *task.EventID != eventID {
		t.Errorf("EventID = %v, want %q", task.EventID, eventID)
	}
	if task.NoteID == nil || *task.NoteID != noteID {
		t.Errorf("NoteID = %v, want %q", task.NoteID, noteID)
	}
}

func TestCreateTask_omittedOptionals_nullInWire(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	task := createTaskViaAPI(t, router, `{"title":"Bare"}`)
	if task.DueDate != nil || task.EventID != nil || task.NoteID != nil {
		t.Fatalf("task = %+v, want null optionals", task)
	}
}

func TestCreateTask_invalidDue_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/tasks",
		`{"title":"X","due_date":"soon"}`)

	status, code, fields := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
	if fields["due_date"] != "must be YYYY-MM-DD" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateTask_unknownEvent_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/tasks",
		`{"title":"X","event_id":"nope"}`)

	_, _, fields := decodeErr(t, rec)
	if fields["event_id"] != "unknown event" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateTask_missingTitle_400WithFields(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/tasks", `{"due_date":"2026-10-15"}`)

	status, code, fields := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
	if fields["title"] != "required" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateTask_unknownBodyField_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/tasks", `{"title":"X","priority":1}`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestCreateTask_malformedJSON_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/tasks", `not-json`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestUpdateTask_ok_partialPatchKeepsDue(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	created := createTaskViaAPI(t, router, `{"title":"Old","due_date":"2026-10-10"}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `{"title":"New"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	task := decodeTask(t, rec)
	if task.Title != "New" {
		t.Errorf("Title = %q, want patched", task.Title)
	}
	if task.DueDate == nil || *task.DueDate != "2026-10-10" {
		t.Errorf("DueDate = %v, want kept", task.DueDate)
	}
}

func TestUpdateTask_statusDone_setsCompletedAt(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	created := createTaskViaAPI(t, router, `{"title":"Finish"}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `{"status":"done"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	task := decodeTask(t, rec)
	if task.Status != "done" {
		t.Errorf("Status = %q, want done", task.Status)
	}
	if task.CompletedAt == nil || *task.CompletedAt == "" {
		t.Errorf("CompletedAt = %v, want set", task.CompletedAt)
	}
}

func TestUpdateTask_statusOpen_clearsCompletedAt(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	created := createTaskViaAPI(t, router, `{"title":"Reopen me"}`)
	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `{"status":"done"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("mark done status = %d", rec.Code)
	}

	rec = httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `{"status":"open"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen status = %d, body %s", rec.Code, rec.Body.String())
	}
	task := decodeTask(t, rec)
	if task.Status != "open" {
		t.Errorf("Status = %q, want open", task.Status)
	}
	if task.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want cleared", task.CompletedAt)
	}
}

func TestUpdateTask_invalidStatus_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	created := createTaskViaAPI(t, router, `{"title":"Seed"}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `{"status":"maybe"}`)
	_, _, fields := decodeErr(t, rec)
	if fields["status"] != "must be open or done" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestUpdateTask_dueEmptyString_clearsStoredDate(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	created := createTaskViaAPI(t, router, `{"title":"Seed","due_date":"2026-10-20"}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID,
		`{"title":"Seed","due_date":"2026-10-25","event_id":"","note_id":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("set due status = %d, body %s", rec.Code, rec.Body.String())
	}
	task := decodeTask(t, rec)
	if task.DueDate == nil || *task.DueDate != "2026-10-25" {
		t.Fatalf("DueDate = %v, want 2026-10-25", task.DueDate)
	}

	rec = httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `{"due_date":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear due status = %d, body %s", rec.Code, rec.Body.String())
	}
	if cleared := decodeTask(t, rec); cleared.DueDate != nil {
		t.Fatalf("DueDate = %v, want null after empty string", cleared.DueDate)
	}
}

func TestUpdateTask_invalidDue_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	created := createTaskViaAPI(t, router, `{"title":"Seed"}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `{"due_date":"soon"}`)
	_, _, fields := decodeErr(t, rec)
	if fields["due_date"] != "must be YYYY-MM-DD" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestUpdateTask_missing_404(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/ghost", `{"title":"X"}`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusNotFound || code != "not_found" {
		t.Fatalf("status=%d code=%q, want 404 not_found", status, code)
	}
}

func TestUpdateTask_malformedJSON_400(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	created := createTaskViaAPI(t, router, `{"title":"Seed"}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `not-json`)
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestDeleteTask_ok_204AndGone(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	created := createTaskViaAPI(t, router, `{"title":"Doomed"}`)

	rec := httpRequest(t, router, http.MethodDelete, "/api/tasks/"+created.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	rec = httpRequest(t, router, http.MethodPatch, "/api/tasks/"+created.ID, `{"title":"X"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("after delete status = %d, want 404", rec.Code)
	}
}

func TestDeleteTask_missing_204Idempotent(t *testing.T) {
	_, router, _, _ := newTestTasksHandler(t)
	rec := httpRequest(t, router, http.MethodDelete, "/api/tasks/ghost", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestDeleteTask_internal_500Envelope(t *testing.T) {
	_, router, _, db := newTestTasksHandler(t)
	if _, err := db.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("drop tasks: %v", err)
	}

	rec := httpRequest(t, router, http.MethodDelete, "/api/tasks/ghost", "")
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

package handler

import (
	"net/http"
	"time"

	"noted/internal/service"

	"github.com/go-chi/chi/v5"
)

// taskRequest is the POST /api/tasks body (API.md). DecodeJSON rejects
// unknown keys, so a drifted frontend shape fails with 400 loudly.
type taskRequest struct {
	Title   string  `json:"title"`
	DueDate *string `json:"due_date"`
	EventID *string `json:"event_id"`
	NoteID  *string `json:"note_id"`
}

// taskPatchRequest: every field optional; absent keeps the stored value,
// "" clears due_date/links, and status drives completed_at (service rule).
type taskPatchRequest struct {
	Title   *string `json:"title"`
	DueDate *string `json:"due_date"`
	EventID *string `json:"event_id"`
	NoteID  *string `json:"note_id"`
	Status  *string `json:"status"`
}

// taskJSON is the wire shape the frontend Task type pins (lib/api.ts).
type taskJSON struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`
	DueDate     *string `json:"due_date"`
	EventID     *string `json:"event_id"`
	NoteID      *string `json:"note_id"`
	CompletedAt *string `json:"completed_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// tasksListResponse pins the list envelope (API.md: {data: []}).
type tasksListResponse struct {
	Data []taskJSON `json:"data"`
}

// ListTasks: ?status=open|done|all (absent = all) and ?due=YYYY-MM-DD —
// query parsing lives here (API.md), storage filters are sqlc narg.
func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var status *string
	switch raw := q.Get("status"); raw {
	case "", "all":
		// Absent and "all" both mean the unfiltered list.
	case "open", "done":
		status = &raw
	default:
		writeFieldError(w, "status", "must be open, done, or all")
		return
	}

	var due *string
	if raw := q.Get("due"); raw != "" {
		if !validDate(raw) {
			writeFieldError(w, "due", "must be YYYY-MM-DD")
			return
		}
		due = &raw
	}

	tasks, err := h.tasks.List(r.Context(), status, due)
	if err != nil {
		writeServiceError(w, err, "list tasks")
		return
	}

	data := make([]taskJSON, len(tasks))
	for i, task := range tasks {
		data[i] = toTaskJSON(task)
	}

	WriteJSON(w, http.StatusOK, tasksListResponse{Data: data})
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req taskRequest
	if !DecodeJSON(w, r, &req) {
		return
	}
	if req.DueDate != nil && *req.DueDate != "" && !validDate(*req.DueDate) {
		writeFieldError(w, "due_date", "must be YYYY-MM-DD")
		return
	}

	task, err := h.tasks.Create(r.Context(), service.TaskInput{
		Title:   req.Title,
		DueDate: req.DueDate,
		EventID: req.EventID,
		NoteID:  req.NoteID,
	})
	if err != nil {
		writeServiceError(w, err, "create task")
		return
	}

	WriteJSON(w, http.StatusCreated, toTaskJSON(task))
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	var req taskPatchRequest
	if !DecodeJSON(w, r, &req) {
		return
	}
	if req.DueDate != nil && *req.DueDate != "" && !validDate(*req.DueDate) {
		writeFieldError(w, "due_date", "must be YYYY-MM-DD")
		return
	}

	task, err := h.tasks.Update(r.Context(), chi.URLParam(r, "id"), service.TaskPatch{
		Title:   req.Title,
		DueDate: req.DueDate,
		EventID: req.EventID,
		NoteID:  req.NoteID,
		Status:  req.Status,
	})
	if err != nil {
		writeServiceError(w, err, "update task")
		return
	}

	WriteJSON(w, http.StatusOK, toTaskJSON(task))
}

// DeleteTask: idempotent 204 — soft-delete stamps deleted_at; purge is
// the only real DELETE, nulling inbound transaction links at that point.
func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	if err := h.tasks.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err, "delete task")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toTaskJSON(t service.Task) taskJSON {
	return taskJSON{
		ID:          t.ID,
		Title:       t.Title,
		Status:      t.Status,
		DueDate:     t.DueDate,
		EventID:     t.EventID,
		NoteID:      t.NoteID,
		CompletedAt: t.CompletedAt,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// validDate: strict YYYY-MM-DD — stdlib Parse range-checks the day
// (2026-02-30 and 2024-02-29 fail) and the layout demands zero-padded
// fields, so no round-trip check is needed.
func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

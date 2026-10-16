package handler

import (
	"net/http"
	"time"

	"noted/internal/service"

	"github.com/go-chi/chi/v5"
)

// eventRequest is the POST /api/events body (API.md). DecodeJSON rejects
// unknown keys, so a drifted frontend shape fails with 400 loudly.
type eventRequest struct {
	Title       string  `json:"title"`
	StartsAt    string  `json:"starts_at"`
	EndsAt      string  `json:"ends_at"`
	Description *string `json:"description"`
	NoteID      *string `json:"note_id"`
}

// eventPatchRequest: every field optional; absent keeps the stored value.
type eventPatchRequest struct {
	Title       *string `json:"title"`
	StartsAt    *string `json:"starts_at"`
	EndsAt      *string `json:"ends_at"`
	Description *string `json:"description"`
	NoteID      *string `json:"note_id"`
}

// eventJSON is the wire shape the frontend Event type pins (lib/api.ts).
type eventJSON struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	StartsAt    string  `json:"starts_at"`
	EndsAt      string  `json:"ends_at"`
	Description *string `json:"description"`
	NoteID      *string `json:"note_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// eventsListResponse pins the list envelope (API.md: {data: []}) instead
// of a naked map whose key could drift silently.
type eventsListResponse struct {
	Data []eventJSON `json:"data"`
}

// ListEvents: from/to are required RFC3339 instants; the grid always
// sends both (CalendarPage's weekBounds). ?trash=1 swaps the range for
// the soft-deleted list (TrashPanel).
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if q.Get("trash") == "1" {
		h.listDeletedEvents(w, r)
		return
	}

	from, ok := parseTimeParam(q.Get("from"))
	if !ok {
		writeFieldError(w, "from", "required RFC3339 datetime")
		return
	}
	to, ok := parseTimeParam(q.Get("to"))
	if !ok {
		writeFieldError(w, "to", "required RFC3339 datetime")
		return
	}

	if !from.Before(to) {
		writeFieldError(w, "from", "must be before to")
		return
	}

	events, err := h.events.List(r.Context(), from, to)
	if err != nil {
		writeServiceError(w, err, "list events")
		return
	}

	data := make([]eventJSON, len(events))
	for i, event := range events {
		data[i] = toEventJSON(event)
	}

	WriteJSON(w, http.StatusOK, eventsListResponse{Data: data})
}

// listDeletedEvents: the ?trash=1 half of ListEvents.
func (h *Handler) listDeletedEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.events.ListDeleted(r.Context())
	if err != nil {
		writeServiceError(w, err, "list deleted events")
		return
	}

	data := make([]eventJSON, len(events))
	for i, event := range events {
		data[i] = toEventJSON(event)
	}

	WriteJSON(w, http.StatusOK, eventsListResponse{Data: data})
}

func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req eventRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	starts, ok := parseTimeParam(req.StartsAt)
	if !ok {
		writeFieldError(w, "starts_at", "invalid RFC3339 datetime")
		return
	}
	ends, ok := parseTimeParam(req.EndsAt)
	if !ok {
		writeFieldError(w, "ends_at", "invalid RFC3339 datetime")
		return
	}

	event, err := h.events.Create(r.Context(), service.EventInput{
		Title:       req.Title,
		StartsAt:    starts,
		EndsAt:      ends,
		Description: req.Description,
		NoteID:      req.NoteID,
	})
	if err != nil {
		writeServiceError(w, err, "create event")
		return
	}

	WriteJSON(w, http.StatusCreated, toEventJSON(event))
}

func (h *Handler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	var req eventPatchRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	patch := service.EventPatch{
		Title:       req.Title,
		Description: req.Description,
		NoteID:      req.NoteID,
	}

	if req.StartsAt != nil {
		starts, ok := parseTimeParam(*req.StartsAt)
		if !ok {
			writeFieldError(w, "starts_at", "invalid RFC3339 datetime")
			return
		}
		patch.StartsAt = &starts
	}
	if req.EndsAt != nil {
		ends, ok := parseTimeParam(*req.EndsAt)
		if !ok {
			writeFieldError(w, "ends_at", "invalid RFC3339 datetime")
			return
		}
		patch.EndsAt = &ends
	}

	event, err := h.events.Update(r.Context(), chi.URLParam(r, "id"), patch)
	if err != nil {
		writeServiceError(w, err, "update event")
		return
	}

	WriteJSON(w, http.StatusOK, toEventJSON(event))
}

// DeleteEvent: idempotent 204 — the service treats a missing id as done.
// Soft-delete: the row keeps its links until purge, so Undo can restore.
func (h *Handler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	if err := h.events.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err, "delete event")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RestoreEvent: clears deleted_at for the Undo toast and TrashPanel.
func (h *Handler) RestoreEvent(w http.ResponseWriter, r *http.Request) {
	event, err := h.events.Restore(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err, "restore event")
		return
	}

	WriteJSON(w, http.StatusOK, toEventJSON(event))
}

// parseTimeParam accepts RFC3339 with or without fractional seconds (the
// frontend sends toISOString); storage normalizes to whole seconds so the
// string-compare range query stays ordered (internal/service formatStored).
func parseTimeParam(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

func toEventJSON(e service.Event) eventJSON {
	return eventJSON{
		ID:          e.ID,
		Title:       e.Title,
		StartsAt:    e.StartsAt,
		EndsAt:      e.EndsAt,
		Description: e.Description,
		NoteID:      e.NoteID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

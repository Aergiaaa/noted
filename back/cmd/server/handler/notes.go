package handler

import (
	"net/http"

	"noted/internal/service"

	"github.com/go-chi/chi/v5"
)

// noteRequest is the POST /api/notes body (API.md). DecodeJSON rejects
// unknown keys, so a drifted frontend shape fails with 400 loudly.
type noteRequest struct {
	Title  string  `json:"title"`
	BodyMD *string `json:"body_md"`
	Pinned *bool   `json:"pinned"`
}

// notePatchRequest: every field optional; absent keeps the stored value.
type notePatchRequest struct {
	Title  *string `json:"title"`
	BodyMD *string `json:"body_md"`
	Pinned *bool   `json:"pinned"`
}

// noteJSON is the wire shape the frontend Note type pins (lib/api.ts).
type noteJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	BodyMD    string `json:"body_md"`
	Pinned    bool   `json:"pinned"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// notesListResponse pins the list envelope (API.md: {data: []}) instead
// of a naked map whose key could drift silently.
type notesListResponse struct {
	Data []noteJSON `json:"data"`
}

// ListNotes: ?pinned=1/0 filters, absent lists everything pinned-first.
func (h *Handler) ListNotes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var pinned *bool
	switch raw := q.Get("pinned"); raw {
	case "":
		// Absent = no filter: the list panel shows every note.
	case "1":
		yes := true
		pinned = &yes
	case "0":
		no := false
		pinned = &no
	default:
		writeFieldError(w, "pinned", "must be 1 or 0")
		return
	}

	notes, err := h.notes.List(r.Context(), pinned)
	if err != nil {
		writeServiceError(w, err, "list notes")
		return
	}

	data := make([]noteJSON, len(notes))
	for i, note := range notes {
		data[i] = toNoteJSON(note)
	}

	WriteJSON(w, http.StatusOK, notesListResponse{Data: data})
}

func (h *Handler) CreateNote(w http.ResponseWriter, r *http.Request) {
	var req noteRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	in := service.NoteInput{Title: req.Title, Pinned: req.Pinned != nil && *req.Pinned}
	if req.BodyMD != nil {
		in.BodyMD = *req.BodyMD
	}

	note, err := h.notes.Create(r.Context(), in)
	if err != nil {
		writeServiceError(w, err, "create note")
		return
	}

	WriteJSON(w, http.StatusCreated, toNoteJSON(note))
}

func (h *Handler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	var req notePatchRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	note, err := h.notes.Update(r.Context(), chi.URLParam(r, "id"), service.NotePatch{
		Title:  req.Title,
		BodyMD: req.BodyMD,
		Pinned: req.Pinned,
	})
	if err != nil {
		writeServiceError(w, err, "update note")
		return
	}

	WriteJSON(w, http.StatusOK, toNoteJSON(note))
}

// DeleteNote: idempotent 204 — soft-delete stamps deleted_at; notes have
// no Undo surface (events own the trash), so boot's purge reaps at 30d.
func (h *Handler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	if err := h.notes.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err, "delete note")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toNoteJSON(n service.Note) noteJSON {
	return noteJSON{
		ID:        n.ID,
		Title:     n.Title,
		BodyMD:    n.BodyMD,
		Pinned:    n.Pinned,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

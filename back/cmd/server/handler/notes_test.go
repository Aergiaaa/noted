package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"noted/internal/database"
	"noted/internal/database/generated"
	"noted/internal/service"

	"github.com/go-chi/chi/v5"
)

// newTestNotesHandler mirrors router.go's route table for the note
// endpoints so {id} params resolve as they do in production.
func newTestNotesHandler(t *testing.T) (*Handler, http.Handler, *service.NoteService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "handler-notes.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := service.NewNoteService(generated.New(db))
	h := New(nil, nil, svc, nil, nil, nil, false)
	r := chi.NewRouter()
	r.Get("/api/notes", h.ListNotes)
	r.Post("/api/notes", h.CreateNote)
	r.Patch("/api/notes/{id}", h.UpdateNote)
	r.Delete("/api/notes/{id}", h.DeleteNote)
	return h, r, svc, db
}

func decodeNote(t *testing.T, rec *httptest.ResponseRecorder) noteJSON {
	t.Helper()
	var note noteJSON
	if err := json.NewDecoder(rec.Body).Decode(&note); err != nil {
		t.Fatalf("decode note: %v", err)
	}
	return note
}

func createNoteViaAPI(t *testing.T, router http.Handler, body string) noteJSON {
	t.Helper()
	rec := httpRequest(t, router, http.MethodPost, "/api/notes", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed status = %d, body %s", rec.Code, rec.Body.String())
	}
	return decodeNote(t, rec)
}

func notesListData(t *testing.T, rec *httptest.ResponseRecorder) []noteJSON {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []noteJSON `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Data
}

func TestListNotes_ok_ordersPinnedFirst(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	createNoteViaAPI(t, router, `{"title":"Plain","body_md":"x","pinned":false}`)
	createNoteViaAPI(t, router, `{"title":"Pinned","body_md":"x","pinned":true}`)

	rec := httpRequest(t, router, http.MethodGet, "/api/notes", "")
	data := notesListData(t, rec)
	if len(data) != 2 || !data[0].Pinned || data[0].Title != "Pinned" {
		t.Fatalf("data = %+v, want the pinned note first", data)
	}
}

func TestListNotes_empty_envelopeNotNull(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodGet, "/api/notes", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"data":[]`) {
		t.Fatalf("body = %s, want an empty array not null", body)
	}
}

func TestListNotes_pinnedParam_filters(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	createNoteViaAPI(t, router, `{"title":"Plain","body_md":"","pinned":false}`)
	createNoteViaAPI(t, router, `{"title":"Pinned","body_md":"","pinned":true}`)

	pinned := notesListData(t, httpRequest(t, router, http.MethodGet, "/api/notes?pinned=1", ""))
	if len(pinned) != 1 || !pinned[0].Pinned {
		t.Fatalf("?pinned=1 = %+v, want only the pinned note", pinned)
	}

	unpinned := notesListData(t, httpRequest(t, router, http.MethodGet, "/api/notes?pinned=0", ""))
	if len(unpinned) != 1 || unpinned[0].Pinned {
		t.Fatalf("?pinned=0 = %+v, want only the unpinned note", unpinned)
	}
}

func TestListNotes_invalidPinned_400(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodGet, "/api/notes?pinned=yes", "")

	status, code, fields := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
	if fields["pinned"] != "must be 1 or 0" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestListNotes_internal_500Envelope(t *testing.T) {
	_, router, _, db := newTestNotesHandler(t)
	if _, err := db.Exec(`DROP TABLE notes`); err != nil {
		t.Fatalf("drop notes: %v", err)
	}

	rec := httpRequest(t, router, http.MethodGet, "/api/notes", "")
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestCreateNote_ok_201_echoesWireShape(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/notes",
		`{"title":"  Plans  ","body_md":"# todo","pinned":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	note := decodeNote(t, rec)
	if note.ID == "" || note.CreatedAt == "" || note.UpdatedAt == "" {
		t.Fatalf("wire shape incomplete: %+v", note)
	}
	if note.Title != "Plans" || note.BodyMD != "# todo" || !note.Pinned {
		t.Fatalf("note = %+v, want trimmed title + stored body/pinned", note)
	}
}

func TestCreateNote_omittedBodyAndPinned_defaults(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	note := createNoteViaAPI(t, router, `{"title":"Bare"}`)
	if note.BodyMD != "" || note.Pinned {
		t.Fatalf("note = %+v, want empty body + unpinned defaults", note)
	}
}

func TestCreateNote_missingTitle_400WithFields(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/notes", `{"body_md":"x"}`)

	status, code, fields := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
	if fields["title"] != "required" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateNote_unknownBodyField_400(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/notes",
		`{"title":"X","color":"red"}`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestCreateNote_malformedJSON_400(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/notes", `not-json`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestCreateNote_bodyTooLong_400(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/notes",
		`{"title":"Long","body_md":"`+strings.Repeat("x", 100001)+`"}`)

	_, _, fields := decodeErr(t, rec)
	if fields["body_md"] != "must be at most 100000 characters" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestUpdateNote_ok_partialPatchKeepsBody(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	created := createNoteViaAPI(t, router, `{"title":"Old","body_md":"keep me","pinned":true}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/notes/"+created.ID, `{"title":"New"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	note := decodeNote(t, rec)
	if note.Title != "New" {
		t.Errorf("Title = %q, want patched", note.Title)
	}
	if note.BodyMD != "keep me" || !note.Pinned {
		t.Errorf("body/pinned = %q/%v, want kept", note.BodyMD, note.Pinned)
	}
}

func TestUpdateNote_pinnedOnlyPatch(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	created := createNoteViaAPI(t, router, `{"title":"Keep","body_md":"x","pinned":false}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/notes/"+created.ID, `{"pinned":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	note := decodeNote(t, rec)
	if !note.Pinned || note.Title != "Keep" {
		t.Fatalf("note = %+v, want pinned flip with title kept", note)
	}
}

func TestUpdateNote_missing_404(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodPatch, "/api/notes/ghost", `{"title":"X"}`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusNotFound || code != "not_found" {
		t.Fatalf("status=%d code=%q, want 404 not_found", status, code)
	}
}

func TestUpdateNote_malformedJSON_400(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	created := createNoteViaAPI(t, router, `{"title":"Seed","body_md":"","pinned":false}`)

	rec := httpRequest(t, router, http.MethodPatch, "/api/notes/"+created.ID, `not-json`)
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestDeleteNote_ok_204AndGone(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	created := createNoteViaAPI(t, router, `{"title":"Doomed","body_md":"","pinned":false}`)

	rec := httpRequest(t, router, http.MethodDelete, "/api/notes/"+created.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	rec = httpRequest(t, router, http.MethodPatch, "/api/notes/"+created.ID, `{"title":"X"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("after delete status = %d, want 404", rec.Code)
	}
}

func TestDeleteNote_missing_204Idempotent(t *testing.T) {
	_, router, _, _ := newTestNotesHandler(t)
	rec := httpRequest(t, router, http.MethodDelete, "/api/notes/ghost", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestDeleteNote_internal_500Envelope(t *testing.T) {
	_, router, _, db := newTestNotesHandler(t)
	if _, err := db.Exec(`DROP TABLE notes`); err != nil {
		t.Fatalf("drop notes: %v", err)
	}

	rec := httpRequest(t, router, http.MethodDelete, "/api/notes/ghost", "")
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

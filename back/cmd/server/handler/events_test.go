package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noted/internal/database"
	"noted/internal/database/generated"
	"noted/internal/service"

	"github.com/go-chi/chi/v5"
)

// newTestEventsHandler wires handler + event service over a migrated
// temp DB; the chi mount mirrors router.go's route table so {id} params
// resolve the same way they do in production.
func newTestEventsHandler(t *testing.T) (*Handler, http.Handler, *service.EventService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "handler-events.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := service.NewEventService(generated.New(db))
	h := New(nil, svc, nil, nil, false)
	r := chi.NewRouter()
	r.Get("/api/events", h.ListEvents)
	r.Post("/api/events", h.CreateEvent)
	r.Patch("/api/events/{id}", h.UpdateEvent)
	r.Delete("/api/events/{id}", h.DeleteEvent)
	r.Post("/api/events/{id}/restore", h.RestoreEvent)
	return h, r, svc, db
}

func httpRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeEvent(t *testing.T, rec *httptest.ResponseRecorder) eventJSON {
	t.Helper()
	var event eventJSON
	if err := json.NewDecoder(rec.Body).Decode(&event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	return event
}

func createViaAPI(t *testing.T, router http.Handler, title string) eventJSON {
	t.Helper()
	rec := httpRequest(t, router, http.MethodPost, "/api/events",
		`{"title":"`+title+`","starts_at":"2026-10-14T09:00:00.000Z","ends_at":"2026-10-14T10:00:00.000Z"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed status = %d, body %s", rec.Code, rec.Body.String())
	}
	return decodeEvent(t, rec)
}

func TestListEvents_ok_orderedByStart(t *testing.T) {
	_, router, svc, _ := newTestEventsHandler(t)
	for _, in := range []service.EventInput{
		{Title: "Later", StartsAt: rfc(t, "2026-10-15T09:00:00Z"), EndsAt: rfc(t, "2026-10-15T10:00:00Z")},
		{Title: "Sooner", StartsAt: rfc(t, "2026-10-14T09:00:00Z"), EndsAt: rfc(t, "2026-10-14T10:00:00Z")},
	} {
		if _, err := svc.Create(context.Background(), in); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	rec := httpRequest(t, router, http.MethodGet,
		"/api/events?from=2026-10-01T00:00:00.000Z&to=2026-10-31T23:59:59.000Z", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []eventJSON `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 2 || body.Data[0].Title != "Sooner" {
		t.Fatalf("data = %+v, want [Sooner Later]", body.Data)
	}
}

func TestListEvents_missingFrom_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodGet, "/api/events?to=2026-10-31T00:00:00Z", "")

	status, code, fields := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
	if fields["from"] != "required RFC3339 datetime" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestListEvents_invalidTo_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodGet, "/api/events?from=2026-10-01T00:00:00Z&to=soon", "")

	_, _, fields := decodeErr(t, rec)
	if fields["to"] != "required RFC3339 datetime" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestListEvents_fromNotBeforeTo_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodGet,
		"/api/events?from=2026-10-02T00:00:00Z&to=2026-10-01T00:00:00Z", "")

	_, _, fields := decodeErr(t, rec)
	if fields["from"] != "must be before to" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestListEvents_internal_500Envelope(t *testing.T) {
	_, router, _, db := newTestEventsHandler(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	rec := httpRequest(t, router, http.MethodGet,
		"/api/events?from=2026-10-01T00:00:00Z&to=2026-10-31T00:00:00Z", "")

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestCreateEvent_ok_201_normalizesTimes(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events",
		`{"title":"Dentist","starts_at":"2026-10-14T09:00:00.000Z","ends_at":"2026-10-14T10:00:00.000Z"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	event := decodeEvent(t, rec)
	if event.ID == "" || event.Title != "Dentist" {
		t.Fatalf("event = %+v", event)
	}
	if event.StartsAt != "2026-10-14T09:00:00Z" || event.EndsAt != "2026-10-14T10:00:00Z" {
		t.Fatalf("times = %q/%q, want whole-second UTC", event.StartsAt, event.EndsAt)
	}
}

func TestCreateEvent_missingTitle_400WithFields(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events",
		`{"title":"","starts_at":"2026-10-14T09:00:00Z","ends_at":"2026-10-14T10:00:00Z"}`)

	status, code, fields := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
	if fields["title"] != "required" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateEvent_endNotAfterStart_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events",
		`{"title":"X","starts_at":"2026-10-14T10:00:00Z","ends_at":"2026-10-14T09:00:00Z"}`)

	_, _, fields := decodeErr(t, rec)
	if fields["ends_at"] != "must be after starts_at" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateEvent_invalidStart_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events",
		`{"title":"X","starts_at":"tomorrow","ends_at":"2026-10-14T10:00:00Z"}`)

	_, _, fields := decodeErr(t, rec)
	if fields["starts_at"] != "invalid RFC3339 datetime" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateEvent_invalidEnd_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events",
		`{"title":"X","starts_at":"2026-10-14T09:00:00Z","ends_at":"soon"}`)

	_, _, fields := decodeErr(t, rec)
	if fields["ends_at"] != "invalid RFC3339 datetime" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateEvent_unknownNote_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events",
		`{"title":"X","starts_at":"2026-10-14T09:00:00Z","ends_at":"2026-10-14T10:00:00Z","note_id":"ghost"}`)

	_, _, fields := decodeErr(t, rec)
	if fields["note_id"] != "unknown note" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreateEvent_unknownBodyField_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events",
		`{"title":"X","starts_at":"2026-10-14T09:00:00Z","ends_at":"2026-10-14T10:00:00Z","color":"red"}`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestCreateEvent_malformedJSON_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events", `not-json`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestUpdateEvent_ok_partialPatch(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	created := createViaAPI(t, router, "Dentist")

	rec := httpRequest(t, router, http.MethodPatch, "/api/events/"+created.ID,
		`{"title":"Eye doctor","starts_at":"2026-10-15T11:00:00Z","ends_at":"2026-10-15T12:00:00Z"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	event := decodeEvent(t, rec)
	if event.Title != "Eye doctor" {
		t.Errorf("Title = %q", event.Title)
	}
	if event.StartsAt != "2026-10-15T11:00:00Z" || event.EndsAt != "2026-10-15T12:00:00Z" {
		t.Errorf("times = %q/%q, want patched values", event.StartsAt, event.EndsAt)
	}
}

func TestUpdateEvent_missing_404(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPatch, "/api/events/ghost", `{"title":"X"}`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusNotFound || code != "not_found" {
		t.Fatalf("status=%d code=%q, want 404 not_found", status, code)
	}
}

func TestUpdateEvent_invalidStart_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	created := createViaAPI(t, router, "Dentist")

	rec := httpRequest(t, router, http.MethodPatch, "/api/events/"+created.ID,
		`{"starts_at":"whenever"}`)
	_, _, fields := decodeErr(t, rec)
	if fields["starts_at"] != "invalid RFC3339 datetime" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestUpdateEvent_invalidEnd_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	created := createViaAPI(t, router, "Dentist")

	rec := httpRequest(t, router, http.MethodPatch, "/api/events/"+created.ID,
		`{"ends_at":"whenever"}`)
	_, _, fields := decodeErr(t, rec)
	if fields["ends_at"] != "invalid RFC3339 datetime" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestUpdateEvent_malformedJSON_400(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	created := createViaAPI(t, router, "Dentist")

	rec := httpRequest(t, router, http.MethodPatch, "/api/events/"+created.ID, `not-json`)
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q", status, code)
	}
}

func TestDeleteEvent_ok_204AndGone(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	created := createViaAPI(t, router, "Dentist")

	rec := httpRequest(t, router, http.MethodDelete, "/api/events/"+created.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	rec = httpRequest(t, router, http.MethodPatch, "/api/events/"+created.ID, `{"title":"X"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("after delete status = %d, want 404", rec.Code)
	}
}

func TestDeleteEvent_missing_204Idempotent(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodDelete, "/api/events/ghost", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestDeleteEvent_internal_500Envelope(t *testing.T) {
	_, router, _, db := newTestEventsHandler(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	rec := httpRequest(t, router, http.MethodDelete, "/api/events/ghost", "")

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestRestoreEvent_ok_200(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	created := createViaAPI(t, router, "Dentist")

	if rec := httpRequest(t, router, http.MethodDelete, "/api/events/"+created.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	rec := httpRequest(t, router, http.MethodPost, "/api/events/"+created.ID+"/restore", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("restore status = %d, body %s", rec.Code, rec.Body.String())
	}
	restored := decodeEvent(t, rec)
	if restored.ID != created.ID || restored.Title != "Dentist" {
		t.Fatalf("restored = %+v, want original row", restored)
	}
}

func TestRestoreEvent_missing_404(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	rec := httpRequest(t, router, http.MethodPost, "/api/events/ghost/restore", "")

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusNotFound || code != "not_found" {
		t.Fatalf("status=%d code=%q, want 404 not_found", status, code)
	}
}

func TestRestoreEvent_internal_500Envelope(t *testing.T) {
	_, router, _, db := newTestEventsHandler(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	rec := httpRequest(t, router, http.MethodPost, "/api/events/ghost/restore", "")

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestListEvents_trash_softDeletedOnly_noRangeNeeded(t *testing.T) {
	_, router, _, _ := newTestEventsHandler(t)
	keep := createViaAPI(t, router, "Keep")
	drop := createViaAPI(t, router, "Drop")

	if rec := httpRequest(t, router, http.MethodDelete, "/api/events/"+drop.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}

	rec := httpRequest(t, router, http.MethodGet, "/api/events?trash=1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("trash status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []eventJSON `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != drop.ID {
		t.Fatalf("trash data = %+v, want only %s", body.Data, drop.ID)
	}

	rec = httpRequest(t, router, http.MethodGet,
		"/api/events?from=2026-10-01T00:00:00.000Z&to=2026-10-31T23:59:59.000Z", "")
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode range: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != keep.ID {
		t.Fatalf("range data = %+v, want only %s", body.Data, keep.ID)
	}
}

func TestListEvents_trash_internal_500Envelope(t *testing.T) {
	_, router, _, db := newTestEventsHandler(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	rec := httpRequest(t, router, http.MethodGet, "/api/events?trash=1", "")

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func rfc(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}

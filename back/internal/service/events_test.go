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

func newTestEventService(t *testing.T) (*EventService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewEventService(generated.New(db)), db
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}

func validInput(t *testing.T) EventInput {
	t.Helper()
	return EventInput{
		Title:    "Dentist",
		StartsAt: mustTime(t, "2026-10-14T09:00:00Z"),
		EndsAt:   mustTime(t, "2026-10-14T10:00:00Z"),
	}
}

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	return validation.Fields
}

func TestEventService_create_persistsNormalizedUTC(t *testing.T) {
	svc, _ := newTestEventService(t)
	description := "bring referral letter"
	noteID := ""

	created, err := svc.Create(context.Background(), EventInput{
		Title:       " Dentist ",
		StartsAt:    mustTime(t, "2026-10-14T09:00:00.500Z"),
		EndsAt:      mustTime(t, "2026-10-14T10:00:00.500Z"),
		Description: &description,
		NoteID:      &noteID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("id/timestamps empty: %+v", created)
	}
	if created.Title != "Dentist" {
		t.Errorf("Title = %q, want trimmed", created.Title)
	}
	if created.StartsAt != "2026-10-14T09:00:00Z" || created.EndsAt != "2026-10-14T10:00:00Z" {
		t.Errorf("times = %q/%q, want whole-second UTC", created.StartsAt, created.EndsAt)
	}
	if created.Description == nil || *created.Description != description {
		t.Errorf("Description = %v, want %q", created.Description, description)
	}
	if created.NoteID != nil {
		t.Errorf("NoteID = %v, want nil (empty string normalizes)", created.NoteID)
	}
}

func TestEventService_create_emptyTitle_400(t *testing.T) {
	svc, _ := newTestEventService(t)
	in := validInput(t)
	in.Title = "   "

	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["title"]; got != "required" {
		t.Fatalf("title field = %q, want required", got)
	}
}

func TestEventService_create_titleTooLong_400(t *testing.T) {
	svc, _ := newTestEventService(t)
	in := validInput(t)
	in.Title = strings.Repeat("x", 201)

	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["title"]; got != "must be at most 200 characters" {
		t.Fatalf("title field = %q", got)
	}
}

func TestEventService_create_stripsControlCharacters(t *testing.T) {
	svc, _ := newTestEventService(t)
	description := "line1\r\nline2\x07"

	created, err := svc.Create(context.Background(), EventInput{
		Title:       "Den\x00t\tist ",
		StartsAt:    mustTime(t, "2026-10-14T09:00:00Z"),
		EndsAt:      mustTime(t, "2026-10-14T10:00:00Z"),
		Description: &description,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Title != "Dentist" {
		t.Errorf("Title = %q, want controls stripped", created.Title)
	}
	if created.Description == nil || *created.Description != "line1\nline2" {
		t.Errorf("Description = %v, want CR normalized, controls stripped", created.Description)
	}
}

func TestEventService_create_controlOnlyTitle_400(t *testing.T) {
	svc, _ := newTestEventService(t)
	in := validInput(t)
	in.Title = "\x00\x07\t\n"

	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["title"]; got != "required" {
		t.Fatalf("title field = %q, want required", got)
	}
}

func TestEventService_create_controlOnlyDescription_storedAsNil(t *testing.T) {
	svc, _ := newTestEventService(t)
	in := validInput(t)
	in.Description = func() *string { s := "\x00\x07"; return &s }()

	created, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Description != nil {
		t.Errorf("Description = %v, want nil after sanitize", created.Description)
	}
}

func TestEventService_create_endNotAfterStart_400(t *testing.T) {
	svc, _ := newTestEventService(t)
	in := validInput(t)
	in.EndsAt = in.StartsAt

	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["ends_at"]; got != "must be after starts_at" {
		t.Fatalf("ends_at field = %q", got)
	}
}

func TestEventService_create_unknownNote_400(t *testing.T) {
	svc, _ := newTestEventService(t)
	missing := "no-such-note"
	in := validInput(t)
	in.NoteID = &missing

	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["note_id"]; got != "unknown note" {
		t.Fatalf("note_id field = %q", got)
	}
}

func TestEventService_create_existingNote_ok(t *testing.T) {
	svc, db := newTestEventService(t)
	_, err := db.Exec(
		`INSERT INTO notes (id, title, body_md, pinned, created_at, updated_at)
		 VALUES ('n1', 'Plan', '', 0, '2026-10-01T00:00:00Z', '2026-10-01T00:00:00Z')`,
	)
	if err != nil {
		t.Fatalf("seed note: %v", err)
	}
	in := validInput(t)
	in.NoteID = ptr("n1")

	created, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.NoteID == nil || *created.NoteID != "n1" {
		t.Fatalf("NoteID = %v, want n1", created.NoteID)
	}
}

func TestEventService_create_noteTableGone_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	if _, err := db.Exec(`DROP TABLE notes`); err != nil {
		t.Fatalf("drop notes: %v", err)
	}
	in := validInput(t)
	in.NoteID = ptr("n1")

	if _, err := svc.Create(context.Background(), in); err == nil {
		t.Fatal("Create succeeded, want internal error")
	}
}

func TestEventService_list_overlapSemantics(t *testing.T) {
	svc, _ := newTestEventService(t)
	spans := validInput(t) // 09:00–10:00, fully inside
	spans.StartsAt = mustTime(t, "2026-10-01T09:00:00Z")
	spans.EndsAt = mustTime(t, "2026-10-01T10:00:00Z")
	spanning := validInput(t) // starts before range, ends inside
	spanning.StartsAt = mustTime(t, "2026-09-30T23:00:00Z")
	spanning.EndsAt = mustTime(t, "2026-10-01T00:30:00Z")
	after := validInput(t) // starts after range
	after.StartsAt = mustTime(t, "2026-10-02T09:00:00Z")
	after.EndsAt = mustTime(t, "2026-10-02T10:00:00Z")
	for _, in := range []EventInput{spans, spanning, after} {
		if _, err := svc.Create(context.Background(), in); err != nil {
			t.Fatalf("seed %q: %v", in.Title, err)
		}
	}

	got, err := svc.List(context.Background(),
		mustTime(t, "2026-10-01T00:00:00Z"), mustTime(t, "2026-10-02T00:00:00Z"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (overlap, not containment)", len(got))
	}
	if got[0].StartsAt != "2026-09-30T23:00:00Z" {
		t.Errorf("order[0] = %q, want earliest starts_at first", got[0].StartsAt)
	}
}

func TestEventService_update_mergesPartial(t *testing.T) {
	svc, db := newTestEventService(t)
	_, err := db.Exec(
		`INSERT INTO notes (id, title, body_md, pinned, created_at, updated_at)
		 VALUES ('n1', 'Plan', '', 0, '2026-10-01T00:00:00Z', '2026-10-01T00:00:00Z')`,
	)
	if err != nil {
		t.Fatalf("seed note: %v", err)
	}
	description := "old"
	noteID := "n1"
	created, err := svc.Create(context.Background(), EventInput{
		Title:       "Dentist",
		StartsAt:    mustTime(t, "2026-10-14T09:00:00Z"),
		EndsAt:      mustTime(t, "2026-10-14T10:00:00Z"),
		Description: &description,
		NoteID:      &noteID,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated, err := svc.Update(context.Background(), created.ID, EventPatch{
		Title:       ptr("Eye doctor"),
		StartsAt:    ptr(mustTime(t, "2026-10-15T11:00:00Z")),
		EndsAt:      ptr(mustTime(t, "2026-10-15T12:00:00Z")),
		Description: ptr("new"),
		NoteID:      ptr(""),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Eye doctor" || updated.StartsAt != "2026-10-15T11:00:00Z" ||
		updated.EndsAt != "2026-10-15T12:00:00Z" {
		t.Errorf("merged fields wrong: %+v", updated)
	}
	if updated.Description == nil || *updated.Description != "new" {
		t.Errorf("Description = %v, want new", updated.Description)
	}
	if updated.NoteID != nil {
		t.Errorf("NoteID = %v, want nil (empty string unlinks)", updated.NoteID)
	}
}

func TestEventService_update_keepsUnpatchedFields(t *testing.T) {
	svc, _ := newTestEventService(t)
	created, err := svc.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated, err := svc.Update(context.Background(), created.ID, EventPatch{Title: ptr("Renamed")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Renamed" {
		t.Errorf("Title = %q", updated.Title)
	}
	if updated.StartsAt != created.StartsAt || updated.EndsAt != created.EndsAt {
		t.Errorf("times changed on a title-only patch: %q/%q", updated.StartsAt, updated.EndsAt)
	}
}

func TestEventService_update_stripsControlCharacters(t *testing.T) {
	svc, _ := newTestEventService(t)
	created, err := svc.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated, err := svc.Update(context.Background(), created.ID, EventPatch{
		Title:       ptr("Den\x00tist\x1b"),
		Description: ptr("clean\x07 me"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Dentist" {
		t.Errorf("Title = %q, want controls stripped", updated.Title)
	}
	if updated.Description == nil || *updated.Description != "clean me" {
		t.Errorf("Description = %v, want controls stripped", updated.Description)
	}
}

func TestEventService_update_invalidPatch_400(t *testing.T) {
	svc, _ := newTestEventService(t)
	created, err := svc.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err = svc.Update(context.Background(), created.ID, EventPatch{
		Title:  ptr(""),
		EndsAt: ptr(mustTime(t, created.StartsAt)),
	})
	if got := fieldsOf(t, err)["title"]; got != "required" {
		t.Fatalf("title field = %q, want required", got)
	}
}

func TestEventService_update_missing_404(t *testing.T) {
	svc, _ := newTestEventService(t)
	if _, err := svc.Update(context.Background(), "ghost", EventPatch{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestEventService_update_eventsTableGone_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	if _, err := svc.Update(context.Background(), "any", EventPatch{}); err == nil {
		t.Fatal("Update succeeded, want internal error")
	}
}

func TestEventService_update_corruptStoredTime_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	_, err := db.Exec(
		`INSERT INTO events (id, title, starts_at, ends_at, created_at, updated_at)
		 VALUES ('bad', 'Broken', 'not-a-time', 'also-bad', '2026-10-01T00:00:00Z', '2026-10-01T00:00:00Z')`,
	)
	if err != nil {
		t.Fatalf("seed corrupt row: %v", err)
	}
	if _, err := svc.Update(context.Background(), "bad", EventPatch{Title: ptr("X")}); err == nil {
		t.Fatal("Update succeeded, want parse error")
	}
}

func TestEventService_delete_keepsLinkUntilPurge_nullsInboundTaskLink(t *testing.T) {
	svc, db := newTestEventService(t)
	created, err := svc.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO tasks (id, title, status, event_id, created_at, updated_at)
		 VALUES ('t1', 'Follow up', 'open', ?, '2026-10-01T00:00:00Z', '2026-10-01T00:00:00Z')`,
		created.ID,
	)
	if err != nil {
		t.Fatalf("seed task: %v", err)
	}

	if err := svc.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	var eventID sql.NullString
	if err := db.QueryRow(`SELECT event_id FROM tasks WHERE id = 't1'`).Scan(&eventID); err != nil {
		t.Fatalf("read task: %v", err)
	}
	if !eventID.Valid {
		t.Fatal("task.event_id nulled by soft-delete, want intact until purge")
	}

	// A now past the retention window crosses the purge cutoff.
	purgeNow := time.Now().Add(SOFT_DELETE_RETENTION + time.Hour)
	if n, err := svc.Purge(context.Background(), purgeNow); err != nil {
		t.Fatalf("Purge: %v", err)
	} else if n != 1 {
		t.Fatalf("Purge rows = %d, want 1", n)
	}
	if err := db.QueryRow(`SELECT event_id FROM tasks WHERE id = 't1'`).Scan(&eventID); err != nil {
		t.Fatalf("re-read task: %v", err)
	}
	if eventID.Valid {
		t.Fatalf("task.event_id = %q, want NULL after purge", eventID.String)
	}
}

func TestEventService_delete_missing_isIdempotent(t *testing.T) {
	svc, _ := newTestEventService(t)
	if err := svc.Delete(context.Background(), "ghost"); err != nil {
		t.Fatalf("Delete ghost: %v", err)
	}
}

func TestEventService_delete_eventsTableGone_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	if err := svc.Delete(context.Background(), "any"); err == nil {
		t.Fatal("Delete succeeded, want internal error")
	}
}

func TestEventService_restore_softDeleted_returnsRow(t *testing.T) {
	svc, _ := newTestEventService(t)
	created, err := svc.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := svc.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	restored, err := svc.Restore(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored.ID != created.ID {
		t.Fatalf("restored id = %q, want %q", restored.ID, created.ID)
	}

	events, err := svc.List(context.Background(),
		mustTime(t, "2026-10-01T00:00:00Z"), mustTime(t, "2026-10-15T00:00:00Z"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("List after restore = %d events, want 1", len(events))
	}
}

func TestEventService_restore_missing_isNotFound(t *testing.T) {
	svc, _ := newTestEventService(t)
	if _, err := svc.Restore(context.Background(), "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Restore ghost err = %v, want ErrNotFound", err)
	}
}

func TestEventService_restore_eventsTableGone_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	if _, err := svc.Restore(context.Background(), "any"); err == nil {
		t.Fatal("Restore succeeded, want internal error")
	}
}

func TestEventService_listDeleted_softDeletedOnly(t *testing.T) {
	svc, _ := newTestEventService(t)
	keep, err := svc.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("seed keep: %v", err)
	}
	drop, err := svc.Create(context.Background(), EventInput{
		Title:    "Drop",
		StartsAt: mustTime(t, "2026-10-20T09:00:00Z"),
		EndsAt:   mustTime(t, "2026-10-20T10:00:00Z"),
	})
	if err != nil {
		t.Fatalf("seed drop: %v", err)
	}
	if err := svc.Delete(context.Background(), drop.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	deleted, err := svc.ListDeleted(context.Background())
	if err != nil {
		t.Fatalf("ListDeleted: %v", err)
	}
	if len(deleted) != 1 || deleted[0].ID != drop.ID {
		t.Fatalf("ListDeleted = %+v, want only %s", deleted, drop.ID)
	}

	live, err := svc.List(context.Background(),
		mustTime(t, "2026-10-01T00:00:00Z"), mustTime(t, "2026-10-31T00:00:00Z"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(live) != 1 || live[0].ID != keep.ID {
		t.Fatalf("List = %+v, want only %s", live, keep.ID)
	}
}

func TestEventService_listDeleted_eventsTableGone_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	if _, err := svc.ListDeleted(context.Background()); err == nil {
		t.Fatal("ListDeleted succeeded, want internal error")
	}
}

func TestEventService_purge_removesOnlyPastCutoff(t *testing.T) {
	svc, db := newTestEventService(t)
	old, err := svc.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("seed old: %v", err)
	}
	recent, err := svc.Create(context.Background(), EventInput{
		Title:    "Recent",
		StartsAt: mustTime(t, "2026-10-20T09:00:00Z"),
		EndsAt:   mustTime(t, "2026-10-20T10:00:00Z"),
	})
	if err != nil {
		t.Fatalf("seed recent: %v", err)
	}
	for _, id := range []string{old.ID, recent.ID} {
		if err := svc.Delete(context.Background(), id); err != nil {
			t.Fatalf("Delete %s: %v", id, err)
		}
	}
	// Backdate one row past the retention window.
	if _, err := db.Exec(
		`UPDATE events SET deleted_at = '2020-01-01T00:00:00Z' WHERE id = ?`, old.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	purgeNow := mustTime(t, "2026-10-15T00:00:00Z")
	n, err := svc.Purge(context.Background(), purgeNow)
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 1 {
		t.Fatalf("Purge rows = %d, want 1", n)
	}

	deleted, err := svc.ListDeleted(context.Background())
	if err != nil {
		t.Fatalf("ListDeleted: %v", err)
	}
	if len(deleted) != 1 || deleted[0].ID != recent.ID {
		t.Fatalf("ListDeleted after purge = %+v, want only %s", deleted, recent.ID)
	}
}

func TestEventService_purge_zeroWhenNothingDue(t *testing.T) {
	svc, _ := newTestEventService(t)
	n, err := svc.Purge(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 0 {
		t.Fatalf("Purge rows = %d, want 0", n)
	}
}

func TestValidationError_message(t *testing.T) {
	err := &ValidationError{Fields: map[string]string{"title": "required"}}
	if err.Error() != "validation failed" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestEventService_list_eventsTableGone_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	if _, err := svc.List(context.Background(),
		mustTime(t, "2026-10-01T00:00:00Z"), mustTime(t, "2026-10-02T00:00:00Z")); err == nil {
		t.Fatal("List succeeded, want internal error")
	}
}

func TestEventService_create_eventsTableGone_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	if _, err := db.Exec(`DROP TABLE events`); err != nil {
		t.Fatalf("drop events: %v", err)
	}
	if _, err := svc.Create(context.Background(), validInput(t)); err == nil {
		t.Fatal("Create succeeded, want internal error")
	}
}

func TestEventService_update_writeConflict_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	created, err := svc.Create(context.Background(), validInput(t))
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err = db.Exec(
		`CREATE TRIGGER fail_update BEFORE UPDATE ON events
		 BEGIN SELECT RAISE(ABORT, 'nope'); END`,
	)
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	if _, err := svc.Update(context.Background(), created.ID, EventPatch{Title: ptr("X")}); err == nil {
		t.Fatal("Update succeeded, want write error")
	}
}

func TestEventService_update_corruptEndsTime_internalError(t *testing.T) {
	svc, db := newTestEventService(t)
	_, err := db.Exec(
		`INSERT INTO events (id, title, starts_at, ends_at, created_at, updated_at)
		 VALUES ('bad-end', 'Broken', '2026-10-01T09:00:00Z', 'nope', '2026-10-01T00:00:00Z', '2026-10-01T00:00:00Z')`,
	)
	if err != nil {
		t.Fatalf("seed corrupt row: %v", err)
	}
	if _, err := svc.Update(context.Background(), "bad-end", EventPatch{Title: ptr("X")}); err == nil {
		t.Fatal("Update succeeded, want parse error")
	}
}

func ptr[T any](v T) *T { return &v }

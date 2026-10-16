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

func newTestNoteService(t *testing.T) (*NoteService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "notes.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewNoteService(generated.New(db)), db
}

func seedNote(t *testing.T, svc *NoteService, title string) Note {
	t.Helper()
	note, err := svc.Create(context.Background(), NoteInput{Title: title, BodyMD: "body"})
	if err != nil {
		t.Fatalf("seed note %q: %v", title, err)
	}
	return note
}

func TestNoteService_create_persistsPinnedAndTrimmedTitle(t *testing.T) {
	svc, _ := newTestNoteService(t)

	created, err := svc.Create(context.Background(), NoteInput{
		Title:  "  Plans  ",
		BodyMD: "# todo",
		Pinned: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("id/timestamps empty: %+v", created)
	}
	if created.Title != "Plans" {
		t.Errorf("Title = %q, want trimmed", created.Title)
	}
	if created.BodyMD != "# todo" || !created.Pinned {
		t.Errorf("body/pinned = %q/%v, want stored as given", created.BodyMD, created.Pinned)
	}
}

func TestNoteService_create_emptyBody_ok(t *testing.T) {
	svc, _ := newTestNoteService(t)

	created, err := svc.Create(context.Background(), NoteInput{Title: "Empty"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.BodyMD != "" {
		t.Errorf("BodyMD = %q, want empty default", created.BodyMD)
	}
	if created.Pinned {
		t.Error("Pinned = true, want false default")
	}
}

func TestNoteService_create_emptyTitle_400(t *testing.T) {
	svc, _ := newTestNoteService(t)

	_, err := svc.Create(context.Background(), NoteInput{Title: "   "})
	if got := fieldsOf(t, err)["title"]; got != "required" {
		t.Fatalf("title field = %q, want required", got)
	}
}

func TestNoteService_create_titleTooLong_400(t *testing.T) {
	svc, _ := newTestNoteService(t)

	_, err := svc.Create(context.Background(), NoteInput{Title: strings.Repeat("x", 201)})
	if got := fieldsOf(t, err)["title"]; got != "must be at most 200 characters" {
		t.Fatalf("title field = %q", got)
	}
}

func TestNoteService_create_bodyTooLong_400(t *testing.T) {
	svc, _ := newTestNoteService(t)

	_, err := svc.Create(context.Background(), NoteInput{
		Title:  "Long",
		BodyMD: strings.Repeat("x", 100001),
	})
	if got := fieldsOf(t, err)["body_md"]; got != "must be at most 100000 characters" {
		t.Fatalf("body_md field = %q", got)
	}
}

func TestNoteService_create_bodyAtLimit_ok(t *testing.T) {
	svc, _ := newTestNoteService(t)

	created, err := svc.Create(context.Background(), NoteInput{
		Title:  "Limit",
		BodyMD: strings.Repeat("x", 100000),
	})
	if err != nil {
		t.Fatalf("Create at body limit: %v", err)
	}
	if len([]rune(created.BodyMD)) != 100000 {
		t.Errorf("body length = %d, want 100000", len([]rune(created.BodyMD)))
	}
}

func TestNoteService_create_noteTableGone_internalError(t *testing.T) {
	svc, db := newTestNoteService(t)
	if _, err := db.Exec(`DROP TABLE notes`); err != nil {
		t.Fatalf("drop notes: %v", err)
	}

	_, err := svc.Create(context.Background(), NoteInput{Title: "X"})
	if err == nil {
		t.Fatal("Create succeeded, want internal error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = %v, want internal error not validation", err)
	}
}

func TestNoteService_list_ordersPinnedFirst(t *testing.T) {
	svc, _ := newTestNoteService(t)
	seedNote(t, svc, "Plain")
	pinned, err := svc.Create(context.Background(), NoteInput{Title: "Pinned", Pinned: true})
	if err != nil {
		t.Fatalf("seed pinned: %v", err)
	}

	notes, err := svc.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 2 || notes[0].ID != pinned.ID {
		t.Fatalf("List = %+v, want pinned note first", notes)
	}
}

func TestNoteService_list_pinnedFilter(t *testing.T) {
	svc, _ := newTestNoteService(t)
	seedNote(t, svc, "Plain")
	pinned, err := svc.Create(context.Background(), NoteInput{Title: "Pinned", Pinned: true})
	if err != nil {
		t.Fatalf("seed pinned: %v", err)
	}
	yes := true
	no := false

	pinnedOnly, err := svc.List(context.Background(), &yes)
	if err != nil {
		t.Fatalf("List pinned: %v", err)
	}
	if len(pinnedOnly) != 1 || pinnedOnly[0].ID != pinned.ID {
		t.Fatalf("List(pinned) = %+v, want only the pinned note", pinnedOnly)
	}

	unpinnedOnly, err := svc.List(context.Background(), &no)
	if err != nil {
		t.Fatalf("List unpinned: %v", err)
	}
	if len(unpinnedOnly) != 1 || unpinnedOnly[0].Title != "Plain" {
		t.Fatalf("List(unpinned) = %+v, want only the plain note", unpinnedOnly)
	}
}

func TestNoteService_list_noteTableGone_internalError(t *testing.T) {
	svc, db := newTestNoteService(t)
	if _, err := db.Exec(`DROP TABLE notes`); err != nil {
		t.Fatalf("drop notes: %v", err)
	}

	if _, err := svc.List(context.Background(), nil); err == nil {
		t.Fatal("List succeeded, want internal error")
	}
}

func TestNoteService_update_mergesPartial(t *testing.T) {
	svc, _ := newTestNoteService(t)
	note := seedNote(t, svc, "Old")

	updated, err := svc.Update(context.Background(), note.ID, NotePatch{Title: ptr("New")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "New" {
		t.Errorf("Title = %q, want patched", updated.Title)
	}
	if updated.BodyMD != "body" {
		t.Errorf("BodyMD = %q, want kept", updated.BodyMD)
	}
	if updated.Pinned {
		t.Error("Pinned flipped, want kept false")
	}
}

func TestNoteService_update_pinnedAndBodyPatch(t *testing.T) {
	svc, _ := newTestNoteService(t)
	note := seedNote(t, svc, "KeepTitle")

	updated, err := svc.Update(context.Background(), note.ID, NotePatch{
		BodyMD: ptr("# new"),
		Pinned: ptr(true),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "KeepTitle" {
		t.Errorf("Title = %q, want kept", updated.Title)
	}
	if updated.BodyMD != "# new" || !updated.Pinned {
		t.Errorf("body/pinned = %q/%v, want patched", updated.BodyMD, updated.Pinned)
	}
}

func TestNoteService_update_missing_404(t *testing.T) {
	svc, _ := newTestNoteService(t)

	_, err := svc.Update(context.Background(), "ghost", NotePatch{Title: ptr("X")})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestNoteService_update_emptyTitle_400(t *testing.T) {
	svc, _ := newTestNoteService(t)
	note := seedNote(t, svc, "Seed")

	_, err := svc.Update(context.Background(), note.ID, NotePatch{Title: ptr("  ")})
	if got := fieldsOf(t, err)["title"]; got != "required" {
		t.Fatalf("title field = %q, want required", got)
	}
}

func TestNoteService_update_noteTableGone_internalError(t *testing.T) {
	svc, db := newTestNoteService(t)
	note := seedNote(t, svc, "Seed")
	if _, err := db.Exec(`DROP TABLE notes`); err != nil {
		t.Fatalf("drop notes: %v", err)
	}

	if _, err := svc.Update(context.Background(), note.ID, NotePatch{Title: ptr("X")}); err == nil {
		t.Fatal("Update succeeded, want internal error")
	}
}

func TestNoteService_update_writeConflict_internalError(t *testing.T) {
	svc, db := newTestNoteService(t)
	note := seedNote(t, svc, "Seed")
	if _, err := db.Exec(
		`CREATE TRIGGER fail_update BEFORE UPDATE ON notes
		 BEGIN SELECT RAISE(ABORT, 'nope'); END`,
	); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	if _, err := svc.Update(context.Background(), note.ID, NotePatch{Title: ptr("X")}); err == nil {
		t.Fatal("Update succeeded, want write error")
	}
}

func TestNoteService_delete_softDeletedLeavesList(t *testing.T) {
	svc, _ := newTestNoteService(t)
	note := seedNote(t, svc, "Doomed")

	if err := svc.Delete(context.Background(), note.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	notes, err := svc.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("List after delete = %+v, want empty", notes)
	}

	if _, err := svc.Update(context.Background(), note.ID, NotePatch{Title: ptr("X")}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update after delete err = %v, want ErrNotFound (deleted rows are gone)", err)
	}
}

func TestNoteService_delete_missing_isIdempotent(t *testing.T) {
	svc, _ := newTestNoteService(t)
	if err := svc.Delete(context.Background(), "ghost"); err != nil {
		t.Fatalf("Delete ghost: %v", err)
	}
}

func TestNoteService_delete_noteTableGone_internalError(t *testing.T) {
	svc, db := newTestNoteService(t)
	if _, err := db.Exec(`DROP TABLE notes`); err != nil {
		t.Fatalf("drop notes: %v", err)
	}

	if err := svc.Delete(context.Background(), "any"); err == nil {
		t.Fatal("Delete succeeded, want internal error")
	}
}

func TestNoteService_delete_purgesExpiredRows(t *testing.T) {
	svc, db := newTestNoteService(t)
	old := seedNote(t, svc, "Old")
	if _, err := db.Exec(
		`UPDATE notes SET deleted_at = '2020-01-01T00:00:00Z' WHERE id = ?`, old.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	fresh := seedNote(t, svc, "Fresh")

	if err := svc.Delete(context.Background(), fresh.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var oldRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM notes WHERE id = ?`, old.ID).Scan(&oldRows); err != nil {
		t.Fatalf("count old: %v", err)
	}
	if oldRows != 0 {
		t.Errorf("expired note rows = %d, want 0 (opportunistic purge)", oldRows)
	}
	var freshRows int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM notes WHERE id = ? AND deleted_at IS NOT NULL`, fresh.ID,
	).Scan(&freshRows); err != nil {
		t.Fatalf("count fresh: %v", err)
	}
	if freshRows != 1 {
		t.Errorf("fresh note rows = %d, want 1 (still soft-deleted)", freshRows)
	}
}

func TestNoteService_purge_removesOnlyPastCutoff(t *testing.T) {
	svc, db := newTestNoteService(t)
	old := seedNote(t, svc, "Old")
	recent := seedNote(t, svc, "Recent")
	for _, id := range []string{old.ID, recent.ID} {
		if err := svc.Delete(context.Background(), id); err != nil {
			t.Fatalf("Delete %s: %v", id, err)
		}
	}
	if _, err := db.Exec(
		`UPDATE notes SET deleted_at = '2020-01-01T00:00:00Z' WHERE id = ?`, old.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	n, err := svc.Purge(context.Background(), mustTime(t, "2026-10-15T00:00:00Z"))
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 1 {
		t.Fatalf("Purge rows = %d, want 1", n)
	}

	var oldRows, recentRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM notes WHERE id = ?`, old.ID).Scan(&oldRows); err != nil {
		t.Fatalf("count old: %v", err)
	}
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM notes WHERE id = ? AND deleted_at IS NOT NULL`, recent.ID,
	).Scan(&recentRows); err != nil {
		t.Fatalf("count recent: %v", err)
	}
	if oldRows != 0 || recentRows != 1 {
		t.Fatalf("rows old/recent = %d/%d, want 0/1", oldRows, recentRows)
	}
}

func TestNoteService_purge_zeroWhenNothingDue(t *testing.T) {
	svc, _ := newTestNoteService(t)
	n, err := svc.Purge(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 0 {
		t.Fatalf("Purge rows = %d, want 0", n)
	}
}

package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"noted/internal/database/generated"
)

// BODY_MAX_CHAR: markdown body ceiling (API.md body_md).
const BODY_MAX_CHAR = 100000

// NoteInput is a fully-resolved note: a create payload, or a PATCH row
// after the service merged it over the stored one.
type NoteInput struct {
	Title  string
	BodyMD string
	Pinned bool
}

// NotePatch: a nil field keeps the stored value (API.md PATCH partial).
type NotePatch struct {
	Title  *string
	BodyMD *string
	Pinned *bool
}

// Note is the domain view of a row; timestamps stay the stored UTC
// strings (already the wire format, like Event).
type Note struct {
	ID        string
	Title     string
	BodyMD    string
	Pinned    bool
	CreatedAt string
	UpdatedAt string
}

type NoteService struct {
	q *generated.Queries
}

func NewNoteService(q *generated.Queries) *NoteService {
	return &NoteService{q: q}
}

// List: nil pinned lists everything, ordered pinned-first by ListNotes
// (pinned DESC, updated_at DESC).
func (s *NoteService) List(ctx context.Context, pinned *bool) ([]Note, error) {
	var filter any
	if pinned != nil {
		if *pinned {
			filter = int64(1)
		} else {
			filter = int64(0)
		}
	}

	rows, err := s.q.ListNotes(ctx, filter)
	if err != nil {
		return nil, err
	}

	notes := make([]Note, len(rows))
	for i, row := range rows {
		notes[i] = fromNoteRow(row)
	}

	return notes, nil
}

func (s *NoteService) Create(ctx context.Context, in NoteInput) (Note, error) {
	in.Title = strings.TrimSpace(in.Title)
	if err := validateNote(in); err != nil {
		return Note{}, err
	}

	now := formatStored(time.Now())
	row, err := s.q.CreateNote(ctx, generated.CreateNoteParams{
		ID:        uuid.NewString(),
		Title:     in.Title,
		BodyMd:    in.BodyMD,
		Pinned:    pinnedInt64(in.Pinned),
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return Note{}, err
	}

	return fromNoteRow(row), nil
}

// Update reads the row, merges the patch over it, re-validates the whole
// (UpdateNote wants every column anyway), and bumps updated_at.
func (s *NoteService) Update(ctx context.Context, id string, patch NotePatch) (Note, error) {
	row, err := s.q.GetNote(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, err
	}

	merged := mergeNotePatch(row, patch)
	merged.Title = strings.TrimSpace(merged.Title)
	if err := validateNote(merged); err != nil {
		return Note{}, err
	}

	updated, err := s.q.UpdateNote(ctx, generated.UpdateNoteParams{
		Title:     merged.Title,
		BodyMd:    merged.BodyMD,
		Pinned:    pinnedInt64(merged.Pinned),
		UpdatedAt: formatStored(time.Now()),
		ID:        id,
	})
	if err != nil {
		return Note{}, err
	}

	return fromNoteRow(updated), nil
}

// Delete is idempotent: it stamps deleted_at (204) and opportunistically
// purges rows past retention; boot retries. Notes have no Undo surface
// (events own the trash), but the retention rule is shared (DOMAIN.md).
func (s *NoteService) Delete(ctx context.Context, id string) error {
	now := formatStored(time.Now())
	if err := s.q.SoftDeleteNote(ctx, generated.SoftDeleteNoteParams{
		DeletedAt: &now,
		ID:        id,
	}); err != nil {
		return err
	}

	s.Purge(ctx, time.Now())

	return nil
}

// Purge hard-deletes rows soft-deleted before the retention cutoff and
// reports how many went. Boot calls it for every domain table.
func (s *NoteService) Purge(ctx context.Context, now time.Time) (int64, error) {
	cutoff := formatStored(now.Add(-SOFT_DELETE_RETENTION))
	return s.q.PurgeNotesDeletedBefore(ctx, &cutoff)
}

func validateNote(in NoteInput) error {
	fields := map[string]string{}
	title := strings.TrimSpace(in.Title)
	switch {
	case title == "":
		fields["title"] = "required"
	case len([]rune(title)) > TITLE_MAX_CHAR:
		fields["title"] = "must be at most 200 characters"
	}

	if len([]rune(in.BodyMD)) > BODY_MAX_CHAR {
		fields["body_md"] = "must be at most 100000 characters"
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	return nil
}

func mergeNotePatch(row generated.Note, patch NotePatch) NoteInput {
	in := NoteInput{
		Title:  row.Title,
		BodyMD: row.BodyMd,
		Pinned: row.Pinned != 0,
	}

	if patch.Title != nil {
		in.Title = *patch.Title
	}
	if patch.BodyMD != nil {
		in.BodyMD = *patch.BodyMD
	}
	if patch.Pinned != nil {
		in.Pinned = *patch.Pinned
	}
	return in
}

// pinnedInt64: the column is INTEGER 0/1 with a CHECK (DATABASE.md).
func pinnedInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func fromNoteRow(row generated.Note) Note {
	return Note{
		ID:        row.ID,
		Title:     row.Title,
		BodyMD:    row.BodyMd,
		Pinned:    row.Pinned != 0,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

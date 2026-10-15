// Package service holds domain rules that outlive HTTP: validation, link
// integrity, and balance math (ARCHITECTURE.md). Handlers never see sqlc
// types — Event is the boundary view.
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

const TITLE_MAX_CHAR = 200

// SOFT_DELETE_RETENTION: how long a soft-deleted row waits before purge
// removes it for real (DOMAIN.md). Boot runs the purge; Delete triggers it
// opportunistically for its own table.
const SOFT_DELETE_RETENTION = 30 * 24 * time.Hour

// ValidationError carries per-field messages straight into the frozen
// 400 envelope (API.md); the handler maps Fields without re-deriving them.
type ValidationError struct {
	Fields map[string]string
}

// just error implementation
func (e *ValidationError) Error() string { return "validation failed" }

// ErrNotFound: the handler maps it to the 404 envelope.
var ErrNotFound = errors.New("not found")

// EventInput is a fully-resolved event: a create payload, or a PATCH row
// after the service merged it over the stored one. Times are instants;
// storage normalizes them to whole-second UTC strings.
type EventInput struct {
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	Description *string
	NoteID      *string
}

// EventPatch: a nil field keeps the stored value (API.md PATCH partial).
type EventPatch struct {
	Title       *string
	StartsAt    *time.Time
	EndsAt      *time.Time
	Description *string
	NoteID      *string
}

// Event is the domain view of a row. Times stay as the stored RFC3339
// UTC strings: they are already the wire format, so the read path never
// re-parses them.
type Event struct {
	ID          string
	Title       string
	StartsAt    string
	EndsAt      string
	Description *string
	NoteID      *string
	CreatedAt   string
	UpdatedAt   string
}

type EventService struct {
	q *generated.Queries
}

func NewEventService(q *generated.Queries) *EventService {
	return &EventService{q: q}
}

func (s *EventService) List(ctx context.Context, from, to time.Time) ([]Event, error) {
	rows, err := s.q.ListEventsBetween(ctx, generated.ListEventsBetweenParams{
		FromTime: formatStored(from),
		ToTime:   formatStored(to),
	})
	if err != nil {
		return nil, err
	}

	events := make([]Event, len(rows))
	for i, row := range rows {
		events[i] = fromRow(row)
	}

	return events, nil
}

func (s *EventService) Create(ctx context.Context, in EventInput) (Event, error) {
	in.Title = strings.TrimSpace(in.Title)
	if err := s.validate(ctx, in); err != nil {
		return Event{}, err
	}

	now := formatStored(time.Now())
	row, err := s.q.CreateEvent(ctx, generated.CreateEventParams{
		ID:          uuid.NewString(),
		Title:       in.Title,
		StartsAt:    formatStored(in.StartsAt),
		EndsAt:      formatStored(in.EndsAt),
		Description: in.Description,
		NoteID:      normalizeID(in.NoteID),
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return Event{}, err
	}

	return fromRow(row), nil
}

// Update reads the row, merges the patch over it, re-validates the whole
// (the full-column UpdateEvent query wants every value anyway), and
// writes back with a bumped updated_at.
func (s *EventService) Update(ctx context.Context, id string, patch EventPatch) (Event, error) {
	row, err := s.q.GetEvent(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, err
	}

	merged, err := mergePatch(row, patch)
	if err != nil {
		return Event{}, err
	}
	merged.Title = strings.TrimSpace(merged.Title)
	if err := s.validate(ctx, merged); err != nil {
		return Event{}, err
	}

	updated, err := s.q.UpdateEvent(ctx, generated.UpdateEventParams{
		ID:          id,
		Title:       merged.Title,
		StartsAt:    formatStored(merged.StartsAt),
		EndsAt:      formatStored(merged.EndsAt),
		Description: merged.Description,
		NoteID:      normalizeID(merged.NoteID),
		UpdatedAt:   formatStored(time.Now()),
	})
	if err != nil {
		return Event{}, err
	}

	return fromRow(updated), nil
}

// Delete is idempotent: it stamps deleted_at (204 backs the Undo toast)
// and opportunistically purges rows past retention. Links stay intact
// until purge, when ON DELETE SET NULL nulls inbound task and transaction
// links (DATABASE.md) — no cascade, peers survive. A purge failure never
// fails the delete; boot retries.
func (s *EventService) Delete(ctx context.Context, id string) error {
	now := formatStored(time.Now())
	if err := s.q.SoftDeleteEvent(ctx, generated.SoftDeleteEventParams{
		DeletedAt: &now,
		ID:        id,
	}); err != nil {
		return err
	}

	s.Purge(ctx, time.Now())

	return nil
}

// Restore clears deleted_at; a missing id is ErrNotFound, a live row is a
// successful no-op (the UPDATE still returns it).
func (s *EventService) Restore(ctx context.Context, id string) (Event, error) {
	row, err := s.q.RestoreEvent(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, err
	}

	return fromRow(row), nil
}

// ListDeleted feeds GET /api/events?trash=1.
func (s *EventService) ListDeleted(ctx context.Context) ([]Event, error) {
	rows, err := s.q.ListDeletedEvents(ctx)
	if err != nil {
		return nil, err
	}

	events := make([]Event, len(rows))
	for i, row := range rows {
		events[i] = fromRow(row)
	}

	return events, nil
}

// Purge hard-deletes rows soft-deleted before the retention cutoff and
// reports how many went. Boot calls it for every domain table.
func (s *EventService) Purge(ctx context.Context, now time.Time) (int64, error) {
	cutoff := formatStored(now.Add(-SOFT_DELETE_RETENTION))
	return s.q.PurgeEventsDeletedBefore(ctx, &cutoff)
}

func (s *EventService) validate(ctx context.Context, in EventInput) error {
	fields := map[string]string{}
	title := strings.TrimSpace(in.Title)
	switch {
	case title == "":
		fields["title"] = "required"
	case len([]rune(title)) > TITLE_MAX_CHAR:
		fields["title"] = "must be at most 200 characters"
	}

	if !in.EndsAt.After(in.StartsAt) {
		fields["ends_at"] = "must be after starts_at"
	}

	if err := s.validateNoteLink(ctx, fields, in.NoteID); err != nil {
		return err
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	return nil
}

// validateNoteLink records "unknown note" for a non-empty link id; a
// storage failure aborts validation with the raw error (writeEventError
// maps it to the generic 500).
func (s *EventService) validateNoteLink(ctx context.Context, fields map[string]string, noteID *string) error {
	id := normalizeID(noteID)
	if id == nil {
		return nil
	}

	_, err := s.q.GetNote(ctx, *id)
	if errors.Is(err, sql.ErrNoRows) {
		fields["note_id"] = "unknown note"
		return nil
	}
	if err != nil {
		return err
	}

	return nil
}

// mergePatch fails only when a stored time is corrupt (parse error) —
// rows we wrote ourselves are always whole-second RFC3339.
func mergePatch(row generated.Event, patch EventPatch) (EventInput, error) {
	starts, err := parseStored(row.StartsAt)
	if err != nil {
		return EventInput{}, err
	}
	ends, err := parseStored(row.EndsAt)
	if err != nil {
		return EventInput{}, err
	}

	in := EventInput{
		Title:       row.Title,
		StartsAt:    starts,
		EndsAt:      ends,
		Description: row.Description,
		NoteID:      row.NoteID,
	}

	if patch.Title != nil {
		in.Title = *patch.Title
	}
	if patch.StartsAt != nil {
		in.StartsAt = *patch.StartsAt
	}
	if patch.EndsAt != nil {
		in.EndsAt = *patch.EndsAt
	}
	if patch.Description != nil {
		in.Description = patch.Description
	}
	if patch.NoteID != nil {
		in.NoteID = patch.NoteID
	}
	return in, nil
}

// formatStored: whole-second UTC so the string-compare range query in
// ListEventsBetween stays ordered — mixed fractional-second formats
// break lexicographic comparison ("00Z" > "00.500Z").
func formatStored(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseStored(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// normalizeID: an empty string means "no link", matching the
// nullable-FK shape the frontend omits when unlinked.
func normalizeID(id *string) *string {
	if id != nil && *id == "" {
		return nil
	}
	return id
}

func fromRow(row generated.Event) Event {
	return Event{
		ID:          row.ID,
		Title:       row.Title,
		StartsAt:    row.StartsAt,
		EndsAt:      row.EndsAt,
		Description: row.Description,
		NoteID:      row.NoteID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

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

const TITLE_MAX = 200

// ValidationError carries per-field messages straight into the frozen
// 400 envelope (API.md); the handler maps Fields without re-deriving them.
type ValidationError struct {
	Fields map[string]string
}

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
		NoteID:      normalizeNoteID(in.NoteID),
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
		NoteID:      normalizeNoteID(merged.NoteID),
		UpdatedAt:   formatStored(time.Now()),
	})
	if err != nil {
		return Event{}, err
	}
	return fromRow(updated), nil
}

// Delete is idempotent: a missing id is still 204 (the :exec query sees
// no rows, not an error). ON DELETE SET NULL nulls inbound task and
// transaction links (DATABASE.md) — no cascade, peers survive.
func (s *EventService) Delete(ctx context.Context, id string) error {
	return s.q.DeleteEvent(ctx, id)
}

func (s *EventService) validate(ctx context.Context, in EventInput) error {
	fields := map[string]string{}
	title := strings.TrimSpace(in.Title)
	switch {
	case title == "":
		fields["title"] = "required"
	case len([]rune(title)) > TITLE_MAX:
		fields["title"] = "must be at most 200 characters"
	}
	if !in.EndsAt.After(in.StartsAt) {
		fields["ends_at"] = "must be after starts_at"
	}
	if noteID := normalizeNoteID(in.NoteID); noteID != nil {
		if _, err := s.q.GetNote(ctx, *noteID); errors.Is(err, sql.ErrNoRows) {
			fields["note_id"] = "unknown note"
		} else if err != nil {
			return err
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
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

// normalizeNoteID: an empty string means "no link", matching the
// nullable-FK shape the frontend omits when unlinked.
func normalizeNoteID(id *string) *string {
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

package modules

import (
	"testing"

	"github.com/Aergiaaa/noted/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBlockChildren(t *testing.T) {
	root := block("a", pgtype.UUID{})
	child := block("b", root.ID)
	grandchild := block("c", child.ID)
	otherRoot := block("d", pgtype.UUID{})

	blocks := []database.GetBlocksByPageRow{root, child, grandchild, otherRoot}

	got := blockChildren(blocks, pgtype.UUID{})
	if len(got) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(got))
	}

	got = blockChildren(blocks, root.ID)
	if len(got) != 1 || got[0].ID != child.ID {
		t.Fatalf("expected 1 child of a, got %+v", got)
	}

	got = blockChildren(blocks, child.ID)
	if len(got) != 1 || got[0].ID != grandchild.ID {
		t.Fatalf("expected 1 child of b, got %+v", got)
	}

	got = blockChildren(blocks, grandchild.ID)
	if len(got) != 0 {
		t.Fatalf("expected 0 children of c, got %d", len(got))
	}
}

func TestBlockTextValue(t *testing.T) {
	for name, tc := range map[string]struct {
		content []byte
		want    string
	}{
		"text":    {content: []byte(`{"text":"hello"}`), want: "hello"},
		"empty":   {content: []byte(`{"text":""}`), want: ""},
		"invalid": {content: []byte(`nope`), want: ""},
		"nil":     {content: nil, want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := blockTextValue(tc.content); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestLinkSegments(t *testing.T) {
	pages := map[string]string{"Alpha": "id-alpha"}

	for name, tc := range map[string]struct {
		text string
		want []textSegment
	}{
		"plain": {
			text: "just text",
			want: []textSegment{{text: "just text"}},
		},
		"single link": {
			text: "see [[Alpha]] now",
			want: []textSegment{
				{text: "see "},
				{text: "Alpha", id: "id-alpha"},
				{text: " now"},
			},
		},
		"unresolved": {
			text: "[[Ghost]]",
			want: []textSegment{{text: "Ghost"}},
		},
		"mixed": {
			text: "a [[Alpha]] b [[Ghost]] c",
			want: []textSegment{
				{text: "a "},
				{text: "Alpha", id: "id-alpha"},
				{text: " b "},
				{text: "Ghost"},
				{text: " c"},
			},
		},
		"double nested brackets": {
			text: "[[a[[b]]c]]",
			want: []textSegment{
				{text: "[[a"},
				{text: "b"},
				{text: "c]]"},
			},
		},
		"empty": {
			text: "",
			want: []textSegment{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := linkSegments(tc.text, pages)

			if len(got) != len(tc.want) {
				t.Fatalf("expected %d segments, got %d: %+v", len(tc.want), len(got), got)
			}

			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("segment %d: expected %+v, got %+v", i, tc.want[i], got[i])
				}
			}
		})
	}
}

func block(id string, parent pgtype.UUID) database.GetBlocksByPageRow {
	uuids := map[string]string{
		"a": "00000000-0000-0000-0000-00000000000a",
		"b": "00000000-0000-0000-0000-00000000000b",
		"c": "00000000-0000-0000-0000-00000000000c",
		"d": "00000000-0000-0000-0000-00000000000d",
	}

	var cleanId pgtype.UUID
	_ = cleanId.Scan(uuids[id])

	return database.GetBlocksByPageRow{
		ID:            cleanId,
		ParentBlockID: parent,
	}
}

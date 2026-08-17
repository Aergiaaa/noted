package service

import (
	"context"
	"errors"
	"testing"
)

func TestBlockServiceCreate(t *testing.T) {
	b := &BlockService{}
	base := CreateBlockArgs{
		PageID:  validUUID,
		Type:    string(BLOCK_TEXT),
		Content: []byte(`{"text":"hi"}`),
	}

	bad := base
	bad.PageID = "nope"
	if _, err := b.Create(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad page id")
	}

	bad = base
	bad.ParentID = "nope"
	if _, err := b.Create(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad parent id")
	}

	bad = base
	bad.Type = "video"
	if _, err := b.Create(context.Background(), bad); !errors.Is(err, ErrBlockKindMismatch) {
		t.Fatalf("expected ErrBlockKindMismatch, got %v", err)
	}
}

func TestBlockServiceUpdate(t *testing.T) {
	b := &BlockService{}
	base := UpdateBlockArgs{
		Id:      validUUID,
		Type:    string(BLOCK_HEADING),
		Content: []byte(`{"text":"hi"}`),
	}

	bad := base
	bad.Id = "nope"
	if _, err := b.Update(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad id")
	}

	bad = base
	bad.Type = "video"
	if _, err := b.Update(context.Background(), bad); !errors.Is(err, ErrBlockKindMismatch) {
		t.Fatalf("expected ErrBlockKindMismatch, got %v", err)
	}
}

func TestBlockServiceUUIDValidation(t *testing.T) {
	b := &BlockService{}

	if err := b.Delete(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if err := b.Restore(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if _, err := b.GetBlocksByPage(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}
}

func TestBlockTypeValid(t *testing.T) {
	for _, v := range []BlockType{BLOCK_TEXT, BLOCK_HEADING, BLOCK_LIST, BLOCK_TABLE, BLOCK_FINANCE} {
		if !blockTypeValid(string(v)) {
			t.Fatalf("expected %q to be valid", v)
		}
	}

	for _, v := range []string{"", "video", "Text", "table "} {
		if blockTypeValid(v) {
			t.Fatalf("expected %q to be invalid", v)
		}
	}
}

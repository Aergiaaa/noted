package service

import (
	"context"
	"errors"
	"testing"
)

func TestEdgeServiceCreate(t *testing.T) {
	e := &EdgeService{}
	base := CreateEdgeArg{
		FromID:   validUUID,
		FromType: string(PAGE),
		ToID:     validUUID,
		ToType:   string(PAGE),
		LinkType: string(EDGE_WIKI_LINK),
	}

	bad := base
	bad.FromID = "nope"
	if err := e.Create(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad from id")
	}

	bad = base
	bad.FromType = "block"
	if err := e.Create(context.Background(), bad); !errors.Is(err, ErrEdgeFromTypeMismatch) {
		t.Fatalf("expected ErrEdgeFromTypeMismatch, got %v", err)
	}

	bad = base
	bad.ToID = "nope"
	if err := e.Create(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad to id")
	}

	bad = base
	bad.ToType = "block"
	if err := e.Create(context.Background(), bad); !errors.Is(err, ErrEdgeToTypeMismatch) {
		t.Fatalf("expected ErrEdgeToTypeMismatch, got %v", err)
	}

	bad = base
	bad.LinkType = "tag"
	if err := e.Create(context.Background(), bad); !errors.Is(err, ErrEdgeLinkTypeMismatch) {
		t.Fatalf("expected ErrEdgeLinkTypeMismatch, got %v", err)
	}
}

func TestEdgeServiceDelete(t *testing.T) {
	e := &EdgeService{}
	base := DeleteEdgeArg{
		FromId:   validUUID,
		ToId:     validUUID,
		LinkType: string(EDGE_PARENT_LINK),
	}

	bad := base
	bad.FromId = "nope"
	if err := e.Delete(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad from id")
	}

	bad = base
	bad.ToId = "nope"
	if err := e.Delete(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad to id")
	}

	bad = base
	bad.LinkType = "tag"
	if err := e.Delete(context.Background(), bad); !errors.Is(err, ErrEdgeLinkTypeMismatch) {
		t.Fatalf("expected ErrEdgeLinkTypeMismatch, got %v", err)
	}
}

func TestEdgeServiceSyncWikiLinks(t *testing.T) {
	e := &EdgeService{}

	if err := e.SyncWikiLinks(context.Background(), "nope", []string{"Alpha"}); err == nil {
		t.Fatalf("expected error for bad page id")
	}
}

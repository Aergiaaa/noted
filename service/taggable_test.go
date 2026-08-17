package service

import (
	"context"
	"errors"
	"testing"
)

func TestTaggableServiceAttach(t *testing.T) {
	tg := &TaggableService{}
	base := AttachTagArg{
		TagID:      validUUID,
		TargetID:   validUUID,
		TargetType: string(PAGE),
	}

	bad := base
	bad.TagID = "nope"
	if err := tg.Attach(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad tag id")
	}

	bad = base
	bad.TargetID = "nope"
	if err := tg.Attach(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad target id")
	}

	bad = base
	bad.TargetType = "block"
	if err := tg.Attach(context.Background(), bad); !errors.Is(err, ErrTagTargetTypeMismatch) {
		t.Fatalf("expected ErrTagTargetTypeMismatch, got %v", err)
	}
}

func TestTaggableServiceDetach(t *testing.T) {
	tg := &TaggableService{}
	base := DetachTagArg{
		TagID:      validUUID,
		TargetID:   validUUID,
		TargetType: string(TRANSACTION),
	}

	bad := base
	bad.TagID = "nope"
	if err := tg.Detach(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad tag id")
	}

	bad = base
	bad.TargetID = "nope"
	if err := tg.Detach(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad target id")
	}

	bad = base
	bad.TargetType = "pageX"
	if err := tg.Detach(context.Background(), bad); !errors.Is(err, ErrTagTargetTypeMismatch) {
		t.Fatalf("expected ErrTagTargetTypeMismatch, got %v", err)
	}
}

func TestTaggableServiceGetTagsByTargetId(t *testing.T) {
	tg := &TaggableService{}

	if _, err := tg.GetTagsByTargetId(context.Background(), "nope", string(PAGE)); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if _, err := tg.GetTagsByTargetId(context.Background(), validUUID, "block"); !errors.Is(err, ErrTagTargetTypeMismatch) {
		t.Fatalf("expected ErrTagTargetTypeMismatch, got %v", err)
	}
}

func TestTaggableServiceGetPagesByTag(t *testing.T) {
	s := &TaggableService{}

	if _, err := s.GetPagesByTag(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad tag id")
	}
}

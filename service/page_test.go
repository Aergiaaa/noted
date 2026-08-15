package service

import (
	"context"
	"errors"
	"testing"
)

func TestPageServiceCreate(t *testing.T) {
	p := &PageService{}

	if _, err := p.Create(context.Background(), "", nil); !errors.Is(err, ErrEmptyTitle) {
		t.Fatalf("expected ErrEmptyTitle, got %v", err)
	}
}

func TestPageServiceUpdate(t *testing.T) {
	p := &PageService{}

	if _, err := p.Update(context.Background(), "", validUUID, nil); !errors.Is(err, ErrEmptyTitle) {
		t.Fatalf("expected ErrEmptyTitle, got %v", err)
	}

	if _, err := p.Update(context.Background(), "Title", "nope", nil); err == nil {
		t.Fatalf("expected error for bad id")
	}
}

func TestPageServiceUUIDValidation(t *testing.T) {
	p := &PageService{}

	if _, err := p.GetById(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if _, err := p.GetBacklinkPages(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if err := p.Delete(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if err := p.Restore(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}
}

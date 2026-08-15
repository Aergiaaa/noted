package service

import (
	"context"
	"errors"
	"testing"
)

func TestTagServiceCreate(t *testing.T) {
	tg := &TagService{}

	if _, err := tg.Create(context.Background(), "", "#fff"); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}

	if _, err := tg.Create(context.Background(), "Work", ""); !errors.Is(err, ErrEmptyColor) {
		t.Fatalf("expected ErrEmptyColor, got %v", err)
	}
}

func TestTagServiceUpdate(t *testing.T) {
	tg := &TagService{}

	if _, err := tg.Update(context.Background(), "", "#fff", validUUID); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}

	if _, err := tg.Update(context.Background(), "Work", "", validUUID); !errors.Is(err, ErrEmptyColor) {
		t.Fatalf("expected ErrEmptyColor, got %v", err)
	}

	if _, err := tg.Update(context.Background(), "Work", "#fff", "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}
}

func TestTagServiceUUIDValidation(t *testing.T) {
	tg := &TagService{}

	if err := tg.Delete(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if err := tg.Restore(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}
}

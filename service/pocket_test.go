package service

import (
	"context"
	"errors"
	"testing"
)

func TestPocketServiceCreate(t *testing.T) {
	p := &PocketService{}

	_, err := p.Create(context.Background(), "", "cash")
	if !errors.Is(err, ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}

	_, err = p.Create(context.Background(), "Wallet", "crypto")
	if !errors.Is(err, ErrPocketKindMismatch) {
		t.Fatalf("expected ErrPocketKindMismatch, got %v", err)
	}
}

func TestPocketServiceUpdate(t *testing.T) {
	p := &PocketService{}

	_, err := p.Update(context.Background(), "", "cash", validUUID)
	if !errors.Is(err, ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}

	_, err = p.Update(context.Background(), "Wallet", "crypto", validUUID)
	if !errors.Is(err, ErrPocketKindMismatch) {
		t.Fatalf("expected ErrPocketKindMismatch, got %v", err)
	}

	_, err = p.Update(context.Background(), "Wallet", "cash", "nope")
	if err == nil {
		t.Fatalf("expected error for bad id")
	}
}

func TestPocketServiceUUIDValidation(t *testing.T) {
	p := &PocketService{}

	if err := p.Delete(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if err := p.Restore(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}
}

const validUUID = "0197f1a0-0000-0000-0000-000000000001"

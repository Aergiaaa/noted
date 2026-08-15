package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTransactionServiceCreate(t *testing.T) {
	tr := &TransactionService{}
	base := CreateTransactionArg{
		Title:  "Gaji",
		Type:   string(TRANSACTION_INCOME),
		Amount: 5000000,
		Date:   time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
	}

	bad := base
	bad.Title = ""
	if _, err := tr.Create(context.Background(), bad); !errors.Is(err, ErrEmptyTitle) {
		t.Fatalf("expected ErrEmptyTitle, got %v", err)
	}

	bad = base
	bad.Type = "loot"
	if _, err := tr.Create(context.Background(), bad); !errors.Is(err, ErrTransactionTypeMismatch) {
		t.Fatalf("expected ErrTransactionTypeMismatch, got %v", err)
	}

	bad = base
	bad.Type = string(TRANSACTION_EXPENSE)
	bad.FromPocketID = "nope"
	if _, err := tr.Create(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad from_pocket_id")
	}

	bad = base
	bad.ToPocketID = "nope"
	if _, err := tr.Create(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad to_pocket_id")
	}
}

func TestTransactionServiceUpdate(t *testing.T) {
	tr := &TransactionService{}
	base := UpdateTransactionArg{
		Title:  "Gaji",
		Type:   string(TRANSACTION_INCOME),
		Amount: 5000000,
		Date:   time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		ID:     validUUID,
	}

	bad := base
	bad.Title = ""
	if _, err := tr.Update(context.Background(), bad); !errors.Is(err, ErrEmptyTitle) {
		t.Fatalf("expected ErrEmptyTitle, got %v", err)
	}

	bad = base
	bad.Type = "loot"
	if _, err := tr.Update(context.Background(), bad); !errors.Is(err, ErrTransactionTypeMismatch) {
		t.Fatalf("expected ErrTransactionTypeMismatch, got %v", err)
	}

	bad = base
	bad.ID = "nope"
	if _, err := tr.Update(context.Background(), bad); err == nil {
		t.Fatalf("expected error for bad id")
	}
}

func TestTransactionServiceUUIDValidation(t *testing.T) {
	tr := &TransactionService{}

	if _, err := tr.GetById(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if err := tr.Delete(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}

	if err := tr.Restore(context.Background(), "nope"); err == nil {
		t.Fatalf("expected error for bad id")
	}
}

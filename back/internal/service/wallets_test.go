package service

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"noted/internal/database"
	"noted/internal/database/generated"
)

func newTestWalletService(t *testing.T) (*WalletService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "wallets.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewWalletService(generated.New(db)), db
}

func seedWallet(t *testing.T, svc *WalletService, name string) Wallet {
	t.Helper()
	wallet, err := svc.Create(context.Background(), name)
	if err != nil {
		t.Fatalf("seed wallet %q: %v", name, err)
	}
	return wallet
}

func TestWalletService_create_persistsTrimmedName(t *testing.T) {
	svc, _ := newTestWalletService(t)

	created, err := svc.Create(context.Background(), "  Bank  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("id/timestamps empty: %+v", created)
	}
	if created.Name != "Bank" {
		t.Errorf("Name = %q, want trimmed", created.Name)
	}
	if created.ArchivedAt != nil {
		t.Errorf("ArchivedAt = %v, want nil", created.ArchivedAt)
	}
}

func TestWalletService_create_emptyName_400(t *testing.T) {
	svc, _ := newTestWalletService(t)

	_, err := svc.Create(context.Background(), "   ")
	if got := fieldsOf(t, err)["name"]; got != "required" {
		t.Fatalf("name field = %q, want required", got)
	}
}

func TestWalletService_create_tooLongName_400(t *testing.T) {
	svc, _ := newTestWalletService(t)

	_, err := svc.Create(context.Background(), strings.Repeat("w", 51))
	if got := fieldsOf(t, err)["name"]; got != "must be at most 50 characters" {
		t.Fatalf("name field = %q, want length message", got)
	}
}

func TestWalletService_list_activeDefaultExcludesArchived(t *testing.T) {
	svc, _ := newTestWalletService(t)
	bank := seedWallet(t, svc, "Bank")

	if _, err := svc.Archive(context.Background(), bank.ID, true); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	active, err := svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 1 || active[0].Name != "Cash" {
		t.Fatalf("active = %+v, want only seeded Cash", active)
	}

	all, err := svc.List(context.Background(), true)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("all = %d wallets, want 2", len(all))
	}
}

func TestWalletService_archiveThenUnarchive(t *testing.T) {
	svc, _ := newTestWalletService(t)
	bank := seedWallet(t, svc, "Bank")

	archived, err := svc.Archive(context.Background(), bank.ID, true)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("ArchivedAt = nil, want stamp")
	}

	live, err := svc.Archive(context.Background(), bank.ID, false)
	if err != nil {
		t.Fatalf("Unarchive: %v", err)
	}
	if live.ArchivedAt != nil {
		t.Errorf("ArchivedAt = %v, want nil after unarchive", live.ArchivedAt)
	}
}

func TestWalletService_rename(t *testing.T) {
	svc, _ := newTestWalletService(t)
	bank := seedWallet(t, svc, "Bank")

	name := "Bank BCA"
	updated, err := svc.Rename(context.Background(), bank.ID, name)
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if updated.Name != "Bank BCA" {
		t.Errorf("Name = %q, want %q", updated.Name, name)
	}
}

func TestWalletService_rename_emptyName_400(t *testing.T) {
	svc, _ := newTestWalletService(t)
	bank := seedWallet(t, svc, "Bank")

	_, err := svc.Rename(context.Background(), bank.ID, " ")
	if got := fieldsOf(t, err)["name"]; got != "required" {
		t.Fatalf("name field = %q, want required", got)
	}
}

func TestWalletService_get_missing_404(t *testing.T) {
	svc, _ := newTestWalletService(t)

	_, err := svc.Get(context.Background(), "wal_missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get = %v, want ErrNotFound", err)
	}
}

func TestWalletService_get_seedCash(t *testing.T) {
	svc, _ := newTestWalletService(t)

	cash, err := svc.Get(context.Background(), WALLET_SEED_CASH_ID)
	if err != nil {
		t.Fatalf("Get seed: %v", err)
	}
	if cash.Name != "Cash" || cash.ArchivedAt != nil {
		t.Errorf("seed = %+v, want live Cash", cash)
	}
}

func TestWalletService_get_dbError(t *testing.T) {
	svc, db := newTestWalletService(t)
	mustExec(t, db, `DROP TABLE wallets`)

	if _, err := svc.Get(context.Background(), WALLET_SEED_CASH_ID); err == nil {
		t.Fatal("Get = nil, want error after dropping wallets")
	}
}

func TestWalletService_list_dbError(t *testing.T) {
	svc, db := newTestWalletService(t)
	mustExec(t, db, `DROP TABLE wallets`)

	if _, err := svc.List(context.Background(), false); err == nil {
		t.Fatal("List = nil, want error after dropping wallets")
	}
}

func TestWalletService_create_dbError(t *testing.T) {
	svc, db := newTestWalletService(t)
	mustExec(t, db, `DROP TABLE wallets`)

	if _, err := svc.Create(context.Background(), "Pot"); err == nil {
		t.Fatal("Create = nil, want error after dropping wallets")
	}
}

func TestWalletService_rename_dbError(t *testing.T) {
	svc, db := newTestWalletService(t)
	mustExec(t, db, `DROP TABLE wallets`)

	if _, err := svc.Rename(context.Background(), WALLET_SEED_CASH_ID, "Cash2"); err == nil {
		t.Fatal("Rename = nil, want error after dropping wallets")
	}
}

func TestWalletService_archive_dbError(t *testing.T) {
	svc, db := newTestWalletService(t)
	mustExec(t, db, `DROP TABLE wallets`)

	if _, err := svc.Archive(context.Background(), WALLET_SEED_CASH_ID, true); err == nil {
		t.Fatal("Archive = nil, want error after dropping wallets")
	}
}

func TestWalletService_renameWrite_dbError(t *testing.T) {
	svc, db := newTestWalletService(t)
	mustExec(t, db, `CREATE TRIGGER trg_wallets_block_update BEFORE UPDATE ON wallets
		BEGIN SELECT RAISE(ABORT, 'blocked'); END`)

	if _, err := svc.Rename(context.Background(), WALLET_SEED_CASH_ID, "Cash2"); err == nil {
		t.Fatal("Rename = nil, want error from blocked write")
	}
}

func TestWalletService_archiveWrite_dbError(t *testing.T) {
	svc, db := newTestWalletService(t)
	mustExec(t, db, `CREATE TRIGGER trg_wallets_block_update BEFORE UPDATE ON wallets
		BEGIN SELECT RAISE(ABORT, 'blocked'); END`)

	if _, err := svc.Archive(context.Background(), WALLET_SEED_CASH_ID, true); err == nil {
		t.Fatal("Archive = nil, want error from blocked write")
	}
}

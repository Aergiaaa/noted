package service

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noted/internal/database"
	"noted/internal/database/generated"
)

func newTestTransactionService(t *testing.T) (*TransactionService, *WalletService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "transactions.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	q := generated.New(db)
	return NewTransactionService(q, "IDR"), NewWalletService(q), db
}

func cashWallet(t *testing.T, wallets *WalletService) Wallet {
	t.Helper()
	cash, err := wallets.Get(context.Background(), WALLET_SEED_CASH_ID)
	if err != nil {
		t.Fatalf("get seed cash: %v", err)
	}
	return cash
}

func incomeInput(walletID string, cents int64, day string) TransactionInput {
	return TransactionInput{
		Kind:        KIND_INCOME,
		AmountCents: cents,
		OccurredOn:  day,
		WalletID:    walletID,
	}
}

func expenseInput(walletID string, cents int64, day string) TransactionInput {
	return TransactionInput{
		Kind:        KIND_EXPENSE,
		AmountCents: cents,
		OccurredOn:  day,
		WalletID:    walletID,
	}
}

func transferInput(from, to string, cents int64, day string) TransactionInput {
	return TransactionInput{
		Kind:         KIND_TRANSFER,
		AmountCents:  cents,
		OccurredOn:   day,
		FromWalletID: from,
		ToWalletID:   to,
	}
}

func TestTransactionService_create_incomeStampsCurrencyAndWallet(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	created, err := svc.Create(context.Background(), incomeInput(cash.ID, 200000, "2026-10-20"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Kind != KIND_INCOME || created.WalletID == nil || *created.WalletID != cash.ID {
		t.Fatalf("kind/wallet = %q/%v, want income on %s", created.Kind, created.WalletID, cash.ID)
	}
	if created.FromWalletID != nil || created.ToWalletID != nil {
		t.Fatalf("transfer columns = %v/%v, want nil", created.FromWalletID, created.ToWalletID)
	}
	if created.Currency != "IDR" {
		t.Errorf("Currency = %q, want stamped IDR", created.Currency)
	}
	if created.AmountCents != 200000 || created.OccurredOn != "2026-10-20" {
		t.Errorf("amount/day = %d/%q", created.AmountCents, created.OccurredOn)
	}
}

func TestTransactionService_create_memoSanitizedAndNilWhenEmpty(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	in := expenseInput(cash.ID, 50000, "2026-10-20")
	memo := "  cof\x00fee  "
	in.Memo = memo
	created, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Memo == nil || *created.Memo != "coffee" {
		t.Fatalf("Memo = %v, want sanitized coffee", created.Memo)
	}

	blank := "   "
	in2 := expenseInput(cash.ID, 1000, "2026-10-20")
	in2.Memo = blank
	created2, err := svc.Create(context.Background(), in2)
	if err != nil {
		t.Fatalf("Create blank memo: %v", err)
	}
	if created2.Memo != nil {
		t.Errorf("Memo = %v, want nil", created2.Memo)
	}
}

func TestTransactionService_create_missingWallet_400(t *testing.T) {
	svc, _, _ := newTestTransactionService(t)

	_, err := svc.Create(context.Background(), incomeInput("", 50000, "2026-10-20"))
	if got := fieldsOf(t, err)["wallet_id"]; got != "required" {
		t.Fatalf("wallet_id field = %q, want required", got)
	}
}

func TestTransactionService_create_unknownWallet_400(t *testing.T) {
	svc, _, _ := newTestTransactionService(t)

	_, err := svc.Create(context.Background(), expenseInput("wal_missing", 50000, "2026-10-20"))
	if got := fieldsOf(t, err)["wallet_id"]; got != "unknown wallet" {
		t.Fatalf("wallet_id field = %q, want unknown wallet", got)
	}
}

func TestTransactionService_create_transferSideColumnsOnExpense_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	in := expenseInput(cash.ID, 50000, "2026-10-20")
	in.FromWalletID = cash.ID
	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["from_wallet_id"]; got != "only transfers use from_wallet_id" {
		t.Fatalf("from_wallet_id field = %q", got)
	}
}

func TestTransactionService_create_amountBounds_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	for _, cents := range []int64{0, -5} {
		_, err := svc.Create(context.Background(), expenseInput(cash.ID, cents, "2026-10-20"))
		if got := fieldsOf(t, err)["amount_cents"]; got != "must be at least 1" {
			t.Fatalf("amount %d field = %q", cents, got)
		}
	}

	_, err := svc.Create(context.Background(), expenseInput(cash.ID, 1e12+1, "2026-10-20"))
	if got := fieldsOf(t, err)["amount_cents"]; got != "must be at most 1000000000000" {
		t.Fatalf("over-max field = %q", got)
	}
}

func TestTransactionService_create_badDate_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	_, err := svc.Create(context.Background(), expenseInput(cash.ID, 50000, "20/10/2026"))
	if got := fieldsOf(t, err)["occurred_on"]; got != "must be YYYY-MM-DD" {
		t.Fatalf("occurred_on field = %q", got)
	}
}

func TestTransactionService_create_badLink_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	cases := []struct {
		name  string
		field string
		want  string
		apply func(in *TransactionInput)
	}{
		{"unknown task", "task_id", "unknown task",
			func(in *TransactionInput) { in.TaskID = "t_missing" }},
		{"unknown event", "event_id", "unknown event",
			func(in *TransactionInput) { in.EventID = "e_missing" }},
		{"unknown note", "note_id", "unknown note",
			func(in *TransactionInput) { in.NoteID = "n_missing" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := expenseInput(cash.ID, 50000, "2026-10-20")
			tc.apply(&in)
			_, err := svc.Create(context.Background(), in)
			if got := fieldsOf(t, err)[tc.field]; got != tc.want {
				t.Fatalf("%s field = %q, want %q", tc.field, got, tc.want)
			}
		})
	}
}

func TestTransactionService_update_badLink_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	created, err := svc.Create(context.Background(), expenseInput(cash.ID, 50000, "2026-10-20"))
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	cases := []struct {
		name  string
		field string
		want  string
		patch TransactionPatch
	}{
		{"unknown task", "task_id", "unknown task",
			TransactionPatch{TaskID: strPtr("t_missing")}},
		{"unknown event", "event_id", "unknown event",
			TransactionPatch{EventID: strPtr("e_missing")}},
		{"unknown note", "note_id", "unknown note",
			TransactionPatch{NoteID: strPtr("n_missing")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Update(context.Background(), created.ID, tc.patch)
			if got := fieldsOf(t, err)[tc.field]; got != tc.want {
				t.Fatalf("%s field = %q, want %q", tc.field, got, tc.want)
			}
		})
	}
}

func TestTransactionService_update_keepsStoredTrashedEventLink(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	eventID := seedEventForTask(t, db)
	task := seedTask(t, NewTaskService(generated.New(db)), TaskInput{Title: "Buy keyboard"})

	in := expenseInput(cash.ID, 50000, "2026-10-20")
	in.EventID = eventID
	in.TaskID = task.ID
	created, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE events SET deleted_at = '2020-01-01T00:00:00Z' WHERE id = ?`, eventID); err != nil {
		t.Fatalf("trash event: %v", err)
	}

	// The chip unlink sends one field; the stored trashed event on the
	// merged row must not re-validate (API.md absent = keep, DOMAIN.md
	// links stay intact until purge).
	unlinked, err := svc.Update(context.Background(), created.ID, TransactionPatch{TaskID: strPtr("")})
	if err != nil {
		t.Fatalf("Update unlink with stored trashed event link: %v", err)
	}
	if unlinked.TaskID != nil {
		t.Fatalf("task_id = %v, want cleared", unlinked.TaskID)
	}
	if unlinked.EventID == nil || *unlinked.EventID != eventID {
		t.Fatalf("event_id = %v, want kept %s", unlinked.EventID, eventID)
	}
}

func TestTransactionService_createTransfer_persistsFromTo(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	bank := seedWallet(t, wallets, "Bank")

	created, err := svc.CreateTransfer(context.Background(), transferInput(cash.ID, bank.ID, 100000, "2026-10-20"))
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	if created.Kind != KIND_TRANSFER {
		t.Errorf("Kind = %q, want transfer", created.Kind)
	}
	if created.WalletID != nil {
		t.Errorf("WalletID = %v, want nil on transfers", created.WalletID)
	}
	if created.FromWalletID == nil || *created.FromWalletID != cash.ID {
		t.Errorf("FromWalletID = %v, want %s", created.FromWalletID, cash.ID)
	}
	if created.ToWalletID == nil || *created.ToWalletID != bank.ID {
		t.Errorf("ToWalletID = %v, want %s", created.ToWalletID, bank.ID)
	}
}

func TestTransactionService_createTransfer_sameWallet_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	_, err := svc.CreateTransfer(context.Background(), transferInput(cash.ID, cash.ID, 1000, "2026-10-20"))
	if got := fieldsOf(t, err)["to_wallet_id"]; got != "must differ from from_wallet_id" {
		t.Fatalf("to_wallet_id field = %q", got)
	}
}

func TestTransactionService_createTransfer_unknownSide_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	_, err := svc.CreateTransfer(context.Background(), transferInput(cash.ID, "wal_missing", 1000, "2026-10-20"))
	if got := fieldsOf(t, err)["to_wallet_id"]; got != "unknown wallet" {
		t.Fatalf("to_wallet_id field = %q", got)
	}
}

func TestTransactionService_createTransfer_missingSides_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	in := transferInput(cash.ID, cash.ID, 1000, "2026-10-20")
	in.ToWalletID = ""
	_, err := svc.CreateTransfer(context.Background(), in)
	if got := fieldsOf(t, err)["to_wallet_id"]; got != "required" {
		t.Fatalf("to_wallet_id field = %q, want required", got)
	}
}

func TestTransactionService_balance_perWalletAndTotal(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	bank := seedWallet(t, wallets, "Bank")

	mustCreate := func(in TransactionInput) {
		t.Helper()
		if _, err := svc.Create(context.Background(), in); err != nil {
			t.Fatalf("seed %s: %v", in.Kind, err)
		}
	}
	mustCreate(incomeInput(cash.ID, 200000, "2026-10-18"))
	mustCreate(expenseInput(cash.ID, 50000, "2026-10-19"))
	if _, err := svc.CreateTransfer(context.Background(), transferInput(bank.ID, cash.ID, 100000, "2026-10-20")); err != nil {
		t.Fatalf("seed transfer: %v", err)
	}

	balance, err := svc.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance.Currency != "IDR" {
		t.Errorf("Currency = %q, want IDR", balance.Currency)
	}
	if balance.TotalCents != 150000 {
		t.Errorf("TotalCents = %d, want 150000 (200000 - 50000; transfers cancel)", balance.TotalCents)
	}

	byID := map[string]int64{}
	for _, w := range balance.Wallets {
		byID[w.WalletID] = w.BalanceCents
	}
	if byID[cash.ID] != 250000 {
		t.Errorf("Cash = %d, want 250000 (200000 - 50000 + 100000)", byID[cash.ID])
	}
	if byID[bank.ID] != -100000 {
		t.Errorf("Bank = %d, want -100000 (transfer out)", byID[bank.ID])
	}
}

func TestTransactionService_balance_voidedDropOut(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	expense, err := svc.Create(context.Background(), expenseInput(cash.ID, 50000, "2026-10-19"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Delete(context.Background(), expense.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	balance, err := svc.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance.TotalCents != 0 || balance.Wallets[0].BalanceCents != 0 {
		t.Errorf("balance after void = %+v, want all zero", balance)
	}
}

func TestTransactionService_delete_voidsAndPurges(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	expense, err := svc.Create(context.Background(), expenseInput(cash.ID, 50000, "2026-10-19"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Delete(context.Background(), expense.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(context.Background(), expense.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after void = %v, want ErrNotFound", err)
	}

	past := formatStored(time.Now().Add(-SOFT_DELETE_RETENTION - time.Hour))
	if _, err := db.Exec(`UPDATE transactions SET deleted_at = ? WHERE id = ?`, past, expense.ID); err != nil {
		t.Fatalf("backdate deleted_at: %v", err)
	}
	purged, err := svc.Purge(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if purged != 1 {
		t.Errorf("Purge = %d, want 1", purged)
	}
}

func TestTransactionService_update_mergesAndKeepsKind(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	created, err := svc.Create(context.Background(), expenseInput(cash.ID, 50000, "2026-10-19"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	memo := "mechanical keyboard"
	newCents := int64(75000)
	updated, err := svc.Update(context.Background(), created.ID, TransactionPatch{
		AmountCents: &newCents,
		Memo:        &memo,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Kind != KIND_EXPENSE || updated.AmountCents != 75000 {
		t.Errorf("kind/amount = %q/%d", updated.Kind, updated.AmountCents)
	}
	if updated.Memo == nil || *updated.Memo != memo {
		t.Errorf("Memo = %v", updated.Memo)
	}
	if updated.WalletID == nil || *updated.WalletID != cash.ID {
		t.Errorf("WalletID = %v, want kept", updated.WalletID)
	}
}

func TestTransactionService_update_movesWalletAndClearsMemo(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	bank := seedWallet(t, wallets, "Bank")

	memo := "wrong pot"
	created, err := svc.Create(context.Background(), func() TransactionInput {
		in := expenseInput(cash.ID, 50000, "2026-10-19")
		in.Memo = memo
		return in
	}())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	empty := ""
	updated, err := svc.Update(context.Background(), created.ID, TransactionPatch{
		WalletID: &bank.ID,
		Memo:     &empty,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.WalletID == nil || *updated.WalletID != bank.ID {
		t.Errorf("WalletID = %v, want %s", updated.WalletID, bank.ID)
	}
	if updated.Memo != nil {
		t.Errorf("Memo = %v, want nil after \"\"", updated.Memo)
	}
}

func TestTransactionService_update_missing_404(t *testing.T) {
	svc, _, _ := newTestTransactionService(t)

	_, err := svc.Update(context.Background(), "tx_missing", TransactionPatch{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update = %v, want ErrNotFound", err)
	}
}

func TestTransactionService_list_pagesSkipNothing(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	days := []string{"2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05"}
	wantIDs := make([]string, 0, len(days))
	for _, day := range days {
		created, err := svc.Create(context.Background(), expenseInput(cash.ID, 1000, day))
		if err != nil {
			t.Fatalf("seed %s: %v", day, err)
		}
		wantIDs = append(wantIDs, created.ID)
	}

	var gotIDs []string
	var cursor *TransactionCursor
	for {
		page, err := svc.List(context.Background(), TransactionListParams{
			Cursor: cursor,
			Limit:  2,
		})
		if err != nil {
			t.Fatalf("List page: %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, row := range page {
			gotIDs = append(gotIDs, row.ID)
		}
		last := page[len(page)-1]
		cursor = &TransactionCursor{OccurredOn: last.OccurredOn, ID: last.ID}
		if len(page) < 2 {
			break
		}
	}

	// Newest first: reversed seed order.
	for i, id := range wantIDs {
		wantIDs[i] = id
	}
	want := []string{wantIDs[4], wantIDs[3], wantIDs[2], wantIDs[1], wantIDs[0]}
	if strings.Join(gotIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("paged ids = %v, want %v", gotIDs, want)
	}
}

func TestTransactionService_list_walletFilterMatchesBothSides(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	bank := seedWallet(t, wallets, "Bank")

	if _, err := svc.Create(context.Background(), expenseInput(bank.ID, 5000, "2026-10-19")); err != nil {
		t.Fatalf("Create expense: %v", err)
	}
	if _, err := svc.CreateTransfer(context.Background(), transferInput(bank.ID, cash.ID, 9000, "2026-10-20")); err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	if _, err := svc.Create(context.Background(), expenseInput(cash.ID, 1000, "2026-10-21")); err != nil {
		t.Fatalf("Create cash expense: %v", err)
	}

	bankRows, err := svc.List(context.Background(), TransactionListParams{WalletID: &bank.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List bank: %v", err)
	}
	if len(bankRows) != 2 {
		t.Fatalf("bank rows = %d, want 2 (expense + transfer side)", len(bankRows))
	}

	cashRows, err := svc.List(context.Background(), TransactionListParams{WalletID: &cash.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List cash: %v", err)
	}
	if len(cashRows) != 2 {
		t.Fatalf("cash rows = %d, want 2 (transfer side + own expense)", len(cashRows))
	}
}

func TestTransactionService_memoTooLong_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	in := expenseInput(cash.ID, 1000, "2026-10-19")
	long := strings.Repeat("m", 501)
	in.Memo = long
	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["memo"]; got != "must be at most 500 characters" {
		t.Fatalf("memo field = %q", got)
	}
}

func TestTransactionService_create_transferKindRejected_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	in := incomeInput(cash.ID, 1000, "2026-10-20")
	in.Kind = KIND_TRANSFER
	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["kind"]; got != "must be income or expense (use /transfers)" {
		t.Fatalf("kind field = %q", got)
	}
}

func TestTransactionService_create_dbError(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	mustExec(t, db, `DROP TABLE transactions`)

	_, err := svc.Create(context.Background(), incomeInput(cash.ID, 1000, "2026-10-20"))
	if err == nil {
		t.Fatal("Create = nil, want error after dropping transactions")
	}
}

func TestTransactionService_get_roundTrip(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	created, err := svc.Create(context.Background(), incomeInput(cash.ID, 4200, "2026-10-20"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID || got.AmountCents != 4200 {
		t.Fatalf("got = %+v, want same row", got)
	}
}

func TestTransactionService_get_dbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `DROP TABLE transactions`)

	if _, err := svc.Get(context.Background(), "tx_x"); err == nil {
		t.Fatal("Get = nil, want error after dropping transactions")
	}
}

func TestTransactionService_list_dbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `DROP TABLE transactions`)

	_, err := svc.List(context.Background(), TransactionListParams{Limit: 10})
	if err == nil {
		t.Fatal("List = nil, want error after dropping transactions")
	}
}

// Balance error exits, one per sum query: each test breaks exactly the
// query whose exit it targets (drop the column only that query reads).

func TestTransactionService_balance_totalsDbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `DROP TABLE transactions`)

	if _, err := svc.Balance(context.Background()); err == nil {
		t.Fatal("Balance = nil, want error after dropping transactions")
	}
}

func TestTransactionService_balance_incomeExpenseSumsDbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `DROP INDEX IF EXISTS transactions_wallet_id_idx; ALTER TABLE transactions DROP COLUMN wallet_id`)

	if _, err := svc.Balance(context.Background()); err == nil {
		t.Fatal("Balance = nil, want error without wallet_id")
	}
}

func TestTransactionService_balance_transferInSumsDbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `ALTER TABLE transactions DROP COLUMN to_wallet_id`)

	if _, err := svc.Balance(context.Background()); err == nil {
		t.Fatal("Balance = nil, want error without to_wallet_id")
	}
}

func TestTransactionService_balance_transferOutSumsDbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `ALTER TABLE transactions DROP COLUMN from_wallet_id`)

	if _, err := svc.Balance(context.Background()); err == nil {
		t.Fatal("Balance = nil, want error without from_wallet_id")
	}
}

func TestTransactionService_balance_walletListDbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `DROP TABLE wallets`)

	if _, err := svc.Balance(context.Background()); err == nil {
		t.Fatal("Balance = nil, want error after dropping wallets")
	}
}

func TestTransactionService_update_amountZero_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	created, err := svc.Create(context.Background(), expenseInput(cash.ID, 1000, "2026-10-20"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	zero := int64(0)
	_, err = svc.Update(context.Background(), created.ID, TransactionPatch{AmountCents: &zero})
	if got := fieldsOf(t, err)["amount_cents"]; got != "must be at least 1" {
		t.Fatalf("amount_cents field = %q", got)
	}
}

func TestTransactionService_update_getDbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `DROP TABLE transactions`)

	_, err := svc.Update(context.Background(), "tx_x", TransactionPatch{})
	if err == nil {
		t.Fatal("Update = nil, want error after dropping transactions")
	}
}

func TestTransactionService_update_writeDbError(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	created, err := svc.Create(context.Background(), incomeInput(cash.ID, 1000, "2026-10-20"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	mustExec(t, db, `CREATE TRIGGER trg_tx_block_update BEFORE UPDATE ON transactions
		BEGIN SELECT RAISE(ABORT, 'blocked'); END`)

	amount := int64(2000)
	if _, err := svc.Update(context.Background(), created.ID, TransactionPatch{AmountCents: &amount}); err == nil {
		t.Fatal("Update = nil, want error from blocked write")
	}
}

func TestTransactionService_update_fullPatchOnLinks(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	task := seedTask(t, NewTaskService(generated.New(db)), TaskInput{Title: "Buy keyboard"})
	eventID := seedEventForTask(t, db)
	noteID := seedNoteForTask(t, db)

	created, err := svc.Create(context.Background(), expenseInput(cash.ID, 1000, "2026-10-20"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	occurred := "2026-10-25"
	updated, err := svc.Update(context.Background(), created.ID, TransactionPatch{
		OccurredOn: &occurred,
		TaskID:     &task.ID,
		EventID:    &eventID,
		NoteID:     &noteID,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.OccurredOn != occurred {
		t.Errorf("OccurredOn = %q, want %q", updated.OccurredOn, occurred)
	}
	if updated.TaskID == nil || *updated.TaskID != task.ID {
		t.Errorf("TaskID = %v, want %s", updated.TaskID, task.ID)
	}
	if updated.EventID == nil || *updated.EventID != eventID {
		t.Errorf("EventID = %v, want %s", updated.EventID, eventID)
	}
	if updated.NoteID == nil || *updated.NoteID != noteID {
		t.Errorf("NoteID = %v, want %s", updated.NoteID, noteID)
	}
}

func TestTransactionService_update_transferSides(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	bank := seedWallet(t, wallets, "Bank")

	created, err := svc.CreateTransfer(context.Background(), transferInput(cash.ID, bank.ID, 1000, "2026-10-20"))
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	updated, err := svc.Update(context.Background(), created.ID, TransactionPatch{
		FromWalletID: &bank.ID,
		ToWalletID:   &cash.ID,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.FromWalletID == nil || *updated.FromWalletID != bank.ID {
		t.Errorf("FromWalletID = %v, want %s", updated.FromWalletID, bank.ID)
	}
	if updated.ToWalletID == nil || *updated.ToWalletID != cash.ID {
		t.Errorf("ToWalletID = %v, want %s", updated.ToWalletID, cash.ID)
	}
}

func TestTransactionService_update_walletOnTransfer_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	bank := seedWallet(t, wallets, "Bank")

	created, err := svc.CreateTransfer(context.Background(), transferInput(cash.ID, bank.ID, 1000, "2026-10-20"))
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	_, err = svc.Update(context.Background(), created.ID, TransactionPatch{WalletID: &cash.ID})
	if got := fieldsOf(t, err)["wallet_id"]; got != "not used by transfers" {
		t.Fatalf("wallet_id field = %q", got)
	}
}

func TestTransactionService_create_incomeToSide_400(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)

	in := incomeInput(cash.ID, 1000, "2026-10-20")
	in.ToWalletID = cash.ID
	_, err := svc.Create(context.Background(), in)
	if got := fieldsOf(t, err)["to_wallet_id"]; got != "only transfers use to_wallet_id" {
		t.Fatalf("to_wallet_id field = %q", got)
	}
}

func TestTransactionService_create_walletRefDbError(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	mustExec(t, db, `DROP TABLE wallets`)

	_, err := svc.Create(context.Background(), incomeInput(cash.ID, 1000, "2026-10-20"))
	if err == nil {
		t.Fatal("Create = nil, want wallet lookup error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = validation %v, want db error", validation.Fields)
	}
}

func TestTransactionService_createTransfer_walletRefDbError(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	bank := seedWallet(t, wallets, "Bank")
	mustExec(t, db, `DROP TABLE wallets`)

	_, err := svc.CreateTransfer(context.Background(), transferInput(cash.ID, bank.ID, 1000, "2026-10-20"))
	if err == nil {
		t.Fatal("CreateTransfer = nil, want wallet lookup error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = validation %v, want db error", validation.Fields)
	}
}

func TestTransactionService_createTransfer_missingFrom_required(t *testing.T) {
	svc, wallets, _ := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	bank := seedWallet(t, wallets, "Bank")

	in := transferInput(cash.ID, bank.ID, 1000, "2026-10-20")
	in.FromWalletID = ""
	_, err := svc.CreateTransfer(context.Background(), in)
	if got := fieldsOf(t, err)["from_wallet_id"]; got != "required" {
		t.Fatalf("from_wallet_id field = %q, want required", got)
	}
}

func TestTransactionService_create_validLinksOk(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	task := seedTask(t, NewTaskService(generated.New(db)), TaskInput{Title: "Buy keyboard"})
	eventID := seedEventForTask(t, db)
	noteID := seedNoteForTask(t, db)

	in := incomeInput(cash.ID, 1000, "2026-10-20")
	in.TaskID = task.ID
	in.EventID = eventID
	in.NoteID = noteID
	created, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create with valid links: %v", err)
	}
	if created.TaskID == nil || created.EventID == nil || created.NoteID == nil {
		t.Fatalf("links = %v/%v/%v, want all set", created.TaskID, created.EventID, created.NoteID)
	}
}

func TestTransactionService_create_taskRefDbError(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	mustExec(t, db, `DROP TABLE tasks`)

	in := incomeInput(cash.ID, 1000, "2026-10-20")
	in.TaskID = "t_any"
	_, err := svc.Create(context.Background(), in)
	if err == nil {
		t.Fatal("Create = nil, want task lookup error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = validation %v, want db error", validation.Fields)
	}
}

func TestTransactionService_create_eventRefDbError(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	mustExec(t, db, `DROP TABLE events`)

	in := incomeInput(cash.ID, 1000, "2026-10-20")
	in.EventID = "e_any"
	_, err := svc.Create(context.Background(), in)
	if err == nil {
		t.Fatal("Create = nil, want event lookup error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = validation %v, want db error", validation.Fields)
	}
}

func TestTransactionService_create_noteRefDbError(t *testing.T) {
	svc, wallets, db := newTestTransactionService(t)
	cash := cashWallet(t, wallets)
	mustExec(t, db, `DROP TABLE notes`)

	in := incomeInput(cash.ID, 1000, "2026-10-20")
	in.NoteID = "n_any"
	_, err := svc.Create(context.Background(), in)
	if err == nil {
		t.Fatal("Create = nil, want note lookup error")
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		t.Fatalf("err = validation %v, want db error", validation.Fields)
	}
}

func TestTransactionService_delete_dbError(t *testing.T) {
	svc, _, db := newTestTransactionService(t)
	mustExec(t, db, `DROP TABLE transactions`)

	if err := svc.Delete(context.Background(), "tx_x"); err == nil {
		t.Fatal("Delete = nil, want error after dropping transactions")
	}
}

func mustExec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatalf("exec %q: %v", stmt, err)
	}
}

func strPtr(s string) *string { return &s }

package handler

import (
	"context"
	"errors"
	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func deletedPocketRow(name string) database.GetDeletedPocketsRow {
	row := database.GetDeletedPocketsRow{Name: name, Type: "cash"}
	_ = row.ID.Scan("0197f1a0-0000-0000-0000-000000000003")
	_ = row.DeletedAt.Scan(time.Now())
	return row
}

func deletedTransactionRow(title string) database.GetDeletedTransactionsRow {
	row := database.GetDeletedTransactionsRow{Title: title, Type: "expense"}
	_ = row.ID.Scan("0197f1a0-0000-0000-0000-000000000004")
	_ = row.Amount.Scan("10")
	_ = row.Date.Scan("2026-08-14")
	_ = row.DeletedAt.Scan(time.Now())
	return row
}

type fakeTransactionServicer struct {
	deleted       []database.GetDeletedTransactionsRow
	getDeletedErr error
	restoreErr    error
	filtered      []database.GetTransactionsWithPocketNamesRow
	filterArg     *service.FilterTransactionsArg
	filterErr     error
	summary       database.GetMonthlySummaryRow
	summaryErr    error
}

func (f *fakeTransactionServicer) Create(ctx context.Context, arg service.CreateTransactionArg) (database.CreateTransactionRow, error) {
	return database.CreateTransactionRow{}, nil
}

func (f *fakeTransactionServicer) Delete(ctx context.Context, id string) error { return nil }

func (f *fakeTransactionServicer) GetAll(ctx context.Context) ([]database.GetAllTransactionsRow, error) {
	return nil, nil
}

func (f *fakeTransactionServicer) GetById(ctx context.Context, id string) (database.GetTransactionByIDRow, error) {
	return database.GetTransactionByIDRow{}, nil
}

func (f *fakeTransactionServicer) GetWithPockets(ctx context.Context) ([]database.GetTransactionsWithPocketNamesRow, error) {
	return nil, nil
}

func (f *fakeTransactionServicer) GetDeleted(ctx context.Context) ([]database.GetDeletedTransactionsRow, error) {
	return f.deleted, f.getDeletedErr
}

func (f *fakeTransactionServicer) GetFiltered(ctx context.Context, arg service.FilterTransactionsArg) ([]database.GetTransactionsWithPocketNamesRow, error) {
	f.filterArg = &arg
	return f.filtered, f.filterErr
}

func (f *fakeTransactionServicer) Restore(ctx context.Context, id string) error { return f.restoreErr }

func (f *fakeTransactionServicer) Update(ctx context.Context, arg service.UpdateTransactionArg) (database.UpdateTransactionRow, error) {
	return database.UpdateTransactionRow{}, nil
}

type fakePocketServicer struct {
	deleted       []database.GetDeletedPocketsRow
	getDeletedErr error
	restoreErr    error
	balances      []database.GetPocketBalancesRow
	balancesErr   error
	balanceCalls  int
}

func (f *fakePocketServicer) Create(ctx context.Context, name, kind string) (database.CreatePocketRow, error) {
	return database.CreatePocketRow{}, nil
}

func (f *fakePocketServicer) Delete(ctx context.Context, id string) error { return nil }

func (f *fakePocketServicer) GetAll(ctx context.Context) ([]database.GetAllPocketsRow, error) {
	return nil, nil
}

func (f *fakePocketServicer) GetBalances(ctx context.Context) ([]database.GetPocketBalancesRow, error) {
	f.balanceCalls++
	return f.balances, f.balancesErr
}

func (f *fakePocketServicer) GetDeleted(ctx context.Context) ([]database.GetDeletedPocketsRow, error) {
	return f.deleted, f.getDeletedErr
}

func (f *fakePocketServicer) Restore(ctx context.Context, id string) error { return f.restoreErr }

func (f *fakePocketServicer) Update(ctx context.Context, name, kind, id string) (database.UpdatePocketRow, error) {
	return database.UpdatePocketRow{}, nil
}

func TestHandleRestoreTransaction(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/transactions/0197f1a0-0000-0000-0000-000000000001/restore", nil)
	w := httptest.NewRecorder()

	h.RestoreTransaction(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleRestoreTransactionError(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{restoreErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/transactions/x/restore", nil)
	w := httptest.NewRecorder()

	h.RestoreTransaction(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleRestorePocket(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pockets/0197f1a0-0000-0000-0000-000000000001/restore", nil)
	w := httptest.NewRecorder()

	h.RestorePocket(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleRestorePocketError(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{restoreErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pockets/x/restore", nil)
	w := httptest.NewRecorder()

	h.RestorePocket(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleTrashFragment(t *testing.T) {
	pockets := []database.GetDeletedPocketsRow{deletedPocketRow("Cash2")}
	transactions := []database.GetDeletedTransactionsRow{deletedTransactionRow("Pindah")}
	h := Handler{Service: &service.Services{
		Pocket:      &fakePocketServicer{deleted: pockets},
		Transaction: &fakeTransactionServicer{deleted: transactions},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/trash", nil)
	w := httptest.NewRecorder()

	h.TrashFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Cash2") || !strings.Contains(body, "Pindah") {
		t.Fatalf("expected deleted pocket and transaction in body, got %s", body)
	}
}

func TestHandleTrashFragmentPocketError(t *testing.T) {
	h := Handler{Service: &service.Services{
		Pocket:      &fakePocketServicer{getDeletedErr: errors.New("boom")},
		Transaction: &fakeTransactionServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/trash", nil)
	w := httptest.NewRecorder()

	h.TrashFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleTrashFragmentTransactionError(t *testing.T) {
	h := Handler{Service: &service.Services{
		Pocket:      &fakePocketServicer{},
		Transaction: &fakeTransactionServicer{getDeletedErr: errors.New("boom")},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/trash", nil)
	w := httptest.NewRecorder()

	h.TrashFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleTransactionsFragmentWithFilters(t *testing.T) {
	fake := &fakeTransactionServicer{
		filtered: []database.GetTransactionsWithPocketNamesRow{deletedTransactionRowToRow()},
	}
	h := Handler{Service: &service.Services{
		Transaction: fake,
		Pocket:      &fakePocketServicer{},
		Taggable:    &fakeTaggableServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?pocket=0197f1a0-0000-0000-0000-000000000001&from=2026-08-01&to=2026-08-15", nil)
	w := httptest.NewRecorder()

	h.TransactionsFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.filterArg == nil {
		t.Fatalf("expected GetFiltered to be called")
	}

	if fake.filterArg.PocketID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected pocket id passthrough, got %q", fake.filterArg.PocketID)
	}

	if fake.filterArg.FromDate.Year() != 2026 || fake.filterArg.FromDate.Month() != 8 || fake.filterArg.FromDate.Day() != 1 {
		t.Fatalf("expected from date parsed, got %v", fake.filterArg.FromDate)
	}

	if fake.filterArg.ToDate.Day() != 15 {
		t.Fatalf("expected to date parsed, got %v", fake.filterArg.ToDate)
	}
}

func TestHandleTransactionsFragmentWithoutFilters(t *testing.T) {
	fake := &fakeTransactionServicer{}
	h := Handler{Service: &service.Services{
		Transaction: fake,
		Pocket:      &fakePocketServicer{},
		Taggable:    &fakeTaggableServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions", nil)
	w := httptest.NewRecorder()

	h.TransactionsFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.filterArg != nil {
		t.Fatalf("expected GetWithPockets path without filters, got %v", fake.filterArg)
	}
}

func TestHandleTransactionsFragmentBadDate(t *testing.T) {
	h := Handler{Service: &service.Services{
		Transaction: &fakeTransactionServicer{},
		Pocket:      &fakePocketServicer{},
		Taggable:    &fakeTaggableServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?from=banana", nil)
	w := httptest.NewRecorder()

	h.TransactionsFragment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleTransactionsFragmentServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{
		Transaction: &fakeTransactionServicer{filterErr: errors.New("boom")},
		Pocket:      &fakePocketServicer{},
		Taggable:    &fakeTaggableServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?from=2026-08-01", nil)
	w := httptest.NewRecorder()

	h.TransactionsFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func deletedTransactionRowToRow() database.GetTransactionsWithPocketNamesRow {
	row := database.GetTransactionsWithPocketNamesRow{Title: "Makan", Type: "expense"}
	_ = row.ID.Scan("0197f1a0-0000-0000-0000-000000000004")
	_ = row.Amount.Scan("10")
	_ = row.Date.Scan("2026-08-14")
	return row
}

func taggedTransactionRow(title string) database.GetTransactionsWithPocketNamesRow {
	row := database.GetTransactionsWithPocketNamesRow{Title: title, Type: "income"}
	_ = row.ID.Scan("0197f1a0-0000-0000-0000-000000000005")
	_ = row.Amount.Scan("10")
	_ = row.Date.Scan("2026-08-14")
	return row
}

func TestHandleTransactionsFragmentRendersTags(t *testing.T) {
	taggable := &fakeTaggableServicer{
		tags: []database.GetTagsByTargetRow{{
			Name:  "Makan",
			Color: "#3b82f6",
		}},
	}
	h := Handler{Service: &service.Services{
		Transaction: &fakeTransactionServicer{
			filtered: []database.GetTransactionsWithPocketNamesRow{taggedTransactionRow("Gaji")},
		},
		Pocket:   &fakePocketServicer{},
		Taggable: taggable,
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?from=2026-08-01", nil)
	w := httptest.NewRecorder()

	h.TransactionsFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if taggable.kind != "transaction" {
		t.Fatalf("expected transaction kind, got %q", taggable.kind)
	}

	if taggable.targetId != "0197f1a0-0000-0000-0000-000000000005" {
		t.Fatalf("expected transaction id, got %q", taggable.targetId)
	}

	if !strings.Contains(w.Body.String(), "Makan") {
		t.Fatalf("expected tag name in body, got %s", w.Body.String())
	}
}

func TestHandleTransactionsFragmentTagsError(t *testing.T) {
	h := Handler{Service: &service.Services{
		Transaction: &fakeTransactionServicer{
			filtered: []database.GetTransactionsWithPocketNamesRow{taggedTransactionRow("Gaji")},
		},
		Pocket: &fakePocketServicer{},
		Taggable: &fakeTaggableServicer{
			tagsErr: errors.New("boom"),
		},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?from=2026-08-01", nil)
	w := httptest.NewRecorder()

	h.TransactionsFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func (f *fakeTransactionServicer) GetMonthlySummary(ctx context.Context) (database.GetMonthlySummaryRow, error) {
	return f.summary, f.summaryErr
}

func TestHandlePocketsFragmentSummary(t *testing.T) {
	summary := database.GetMonthlySummaryRow{}
	_ = summary.Income.Scan("100")
	_ = summary.Expense.Scan("40")
	_ = summary.Transfer.Scan("0")

	h := Handler{Service: &service.Services{
		Pocket:      &fakePocketServicer{},
		Transaction: &fakeTransactionServicer{summary: summary},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/pockets", nil)
	w := httptest.NewRecorder()

	h.PocketsFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "100.00") {
		t.Fatalf("expected income in body, got %s", w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "40.00") {
		t.Fatalf("expected expense in body, got %s", w.Body.String())
	}
}

func TestHandlePocketsFragmentSummaryError(t *testing.T) {
	h := Handler{Service: &service.Services{
		Pocket:      &fakePocketServicer{},
		Transaction: &fakeTransactionServicer{summaryErr: errors.New("boom")},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/pockets", nil)
	w := httptest.NewRecorder()

	h.PocketsFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

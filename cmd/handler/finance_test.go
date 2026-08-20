package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/service"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	all           []database.GetAllTransactionsRow
	allErr        error
	byId          database.GetTransactionByIDRow
	byIdErr       error
	createRow     database.CreateTransactionRow
	createErr     error
	createArg     *service.CreateTransactionArg
	updateRow     database.UpdateTransactionRow
	updateErr     error
	updateArg     *service.UpdateTransactionArg
	deleteErr     error
}

func (f *fakeTransactionServicer) Create(ctx context.Context, arg service.CreateTransactionArg) (database.CreateTransactionRow, error) {
	f.createArg = &arg
	return f.createRow, f.createErr
}

func (f *fakeTransactionServicer) Delete(ctx context.Context, id string) error { return f.deleteErr }

func (f *fakeTransactionServicer) GetAll(ctx context.Context) ([]database.GetAllTransactionsRow, error) {
	return f.all, f.allErr
}

func (f *fakeTransactionServicer) GetById(ctx context.Context, id string) (database.GetTransactionByIDRow, error) {
	return f.byId, f.byIdErr
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
	f.updateArg = &arg
	return f.updateRow, f.updateErr
}

type fakePocketServicer struct {
	deleted       []database.GetDeletedPocketsRow
	getDeletedErr error
	restoreErr    error
	balances      []database.GetPocketBalancesRow
	balancesErr   error
	balanceCalls  int
	all           []database.GetAllPocketsRow
	allErr        error
	createRow     database.CreatePocketRow
	createErr     error
	updateRow     database.UpdatePocketRow
	updateErr     error
	deleteErr     error
}

func (f *fakePocketServicer) Create(ctx context.Context, name, kind string) (database.CreatePocketRow, error) {
	return f.createRow, f.createErr
}

func (f *fakePocketServicer) Delete(ctx context.Context, id string) error { return f.deleteErr }

func (f *fakePocketServicer) GetAll(ctx context.Context) ([]database.GetAllPocketsRow, error) {
	return f.all, f.allErr
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
	return f.updateRow, f.updateErr
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

func TestHandleTransactions(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{
		all: []database.GetAllTransactionsRow{{
			Title: "Makan",
			Type:  "expense",
		}},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/transactions", nil)
	w := httptest.NewRecorder()

	h.Transactions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		Transactions []database.GetAllTransactionsRow `json:"transactions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(res.Transactions) != 1 || res.Transactions[0].Title != "Makan" {
		t.Fatalf("expected transaction in response, got %+v", res.Transactions)
	}
}

func TestHandleTransactionsError(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{allErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/transactions", nil)
	w := httptest.NewRecorder()

	h.Transactions(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleGetTransaction(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{
		byId: database.GetTransactionByIDRow{ID: id, Title: "Makan", Type: "expense"},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/transactions/0197f1a0-0000-0000-0000-000000000001", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.ID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected id, got %q", res.ID)
	}
	if res.Title != "Makan" {
		t.Fatalf("expected title, got %q", res.Title)
	}
}

func TestHandleGetTransactionError(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{byIdErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/transactions/x", nil)
	w := httptest.NewRecorder()

	h.GetTransaction(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleCreateTransaction(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	fake := &fakeTransactionServicer{
		createRow: database.CreateTransactionRow{ID: id, Title: "Makan", Type: "expense"},
	}
	h := Handler{Service: &service.Services{Transaction: fake}}

	req := httptest.NewRequest(http.MethodPost, "/api/transactions", bytes.NewBufferString(`{"title":"Makan","type":"expense","amount":25.5,"date":"2026-08-14","from_pocket_id":"0197f1a0-0000-0000-0000-000000000002","to_pocket_id":""}`))
	w := httptest.NewRecorder()

	h.CreateTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.createArg == nil {
		t.Fatalf("expected Create to be called")
	}

	if fake.createArg.Title != "Makan" || fake.createArg.Type != "expense" {
		t.Fatalf("expected title and type passthrough, got %+v", fake.createArg)
	}

	if fake.createArg.Amount != 25.5 {
		t.Fatalf("expected amount passthrough, got %v", fake.createArg.Amount)
	}

	if fake.createArg.Date.Year() != 2026 || fake.createArg.Date.Month() != 8 || fake.createArg.Date.Day() != 14 {
		t.Fatalf("expected date parsed, got %v", fake.createArg.Date)
	}

	if fake.createArg.FromPocketID != "0197f1a0-0000-0000-0000-000000000002" {
		t.Fatalf("expected from pocket passthrough, got %q", fake.createArg.FromPocketID)
	}

	var res struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.ID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected id, got %q", res.ID)
	}
	if res.Title != "Makan" {
		t.Fatalf("expected title, got %q", res.Title)
	}
}

func TestHandleCreateTransactionBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/transactions", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.CreateTransaction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateTransactionBadDate(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/transactions", bytes.NewBufferString(`{"title":"Makan","date":"banana"}`))
	w := httptest.NewRecorder()

	h.CreateTransaction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateTransactionServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{createErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/transactions", bytes.NewBufferString(`{"title":"Makan","date":"2026-08-14"}`))
	w := httptest.NewRecorder()

	h.CreateTransaction(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleUpdateTransaction(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	fake := &fakeTransactionServicer{
		updateRow: database.UpdateTransactionRow{ID: id, Title: "Makan", Type: "expense"},
	}
	h := Handler{Service: &service.Services{Transaction: fake}}

	req := httptest.NewRequest(http.MethodPatch, "/api/transactions/0197f1a0-0000-0000-0000-000000000001", bytes.NewBufferString(`{"title":"Makan","type":"expense","amount":25.5,"date":"2026-08-14"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.updateArg == nil {
		t.Fatalf("expected Update to be called")
	}

	if fake.updateArg.ID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected transaction id passthrough, got %q", fake.updateArg.ID)
	}

	if fake.updateArg.Title != "Makan" {
		t.Fatalf("expected title passthrough, got %q", fake.updateArg.Title)
	}
}

func TestHandleUpdateTransactionBadDate(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/transactions/x", bytes.NewBufferString(`{"title":"Makan","date":"banana"}`))
	w := httptest.NewRecorder()

	h.UpdateTransaction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateTransactionServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{updateErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/transactions/x", bytes.NewBufferString(`{"title":"Makan","date":"2026-08-14"}`))
	w := httptest.NewRecorder()

	h.UpdateTransaction(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleDeleteTransaction(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/transactions/0197f1a0-0000-0000-0000-000000000001", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteTransaction(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleDeleteTransactionError(t *testing.T) {
	h := Handler{Service: &service.Services{Transaction: &fakeTransactionServicer{deleteErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/transactions/x", nil)
	w := httptest.NewRecorder()

	h.DeleteTransaction(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandlePockets(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{
		all: []database.GetAllPocketsRow{{
			ID:   mustUUID("0197f1a0-0000-0000-0000-000000000001"),
			Name: "Wallet Utama",
			Type: "bank",
		}},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pockets", nil)
	w := httptest.NewRecorder()

	h.Pockets(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		Pockets []database.GetAllPocketsRow `json:"pockets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(res.Pockets) != 1 || res.Pockets[0].Name != "Wallet Utama" {
		t.Fatalf("expected pocket in response, got %+v", res.Pockets)
	}
}

func TestHandlePocketsError(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{allErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pockets", nil)
	w := httptest.NewRecorder()

	h.Pockets(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleCreatePocket(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{
		createRow: database.CreatePocketRow{
			ID:   mustUUID("0197f1a0-0000-0000-0000-000000000001"),
			Name: "Wallet Baru",
			Type: "ewallet",
		},
	}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pockets", bytes.NewBufferString(`{"name":"Wallet Baru","type":"ewallet"}`))
	w := httptest.NewRecorder()

	h.CreatePocket(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.ID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected id, got %q", res.ID)
	}
	if res.Name != "Wallet Baru" || res.Type != "ewallet" {
		t.Fatalf("expected name and type, got %s/%s", res.Name, res.Type)
	}
}

func TestHandleCreatePocketBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pockets", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.CreatePocket(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreatePocketServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{createErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pockets", bytes.NewBufferString(`{"name":"Wallet Baru"}`))
	w := httptest.NewRecorder()

	h.CreatePocket(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleUpdatePocket(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{
		updateRow: database.UpdatePocketRow{
			ID:   mustUUID("0197f1a0-0000-0000-0000-000000000001"),
			Name: "Wallet Baru",
			Type: "bank",
		},
	}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/pockets/0197f1a0-0000-0000-0000-000000000001", bytes.NewBufferString(`{"name":"Wallet Baru","type":"bank"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdatePocket(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.Name != "Wallet Baru" {
		t.Fatalf("expected name, got %q", res.Name)
	}
}

func TestHandleUpdatePocketBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/pockets/x", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.UpdatePocket(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdatePocketServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{updateErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/pockets/x", bytes.NewBufferString(`{"name":"Wallet Baru"}`))
	w := httptest.NewRecorder()

	h.UpdatePocket(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleDeletePocket(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/pockets/0197f1a0-0000-0000-0000-000000000001", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeletePocket(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleDeletePocketError(t *testing.T) {
	h := Handler{Service: &service.Services{Pocket: &fakePocketServicer{deleteErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/pockets/x", nil)
	w := httptest.NewRecorder()

	h.DeletePocket(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func mustUUID(s string) pgtype.UUID {
	var id pgtype.UUID
	_ = id.Scan(s)
	return id
}

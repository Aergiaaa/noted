package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/service"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakePageServicer struct {
	create    func(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error)
	updated   bool
	searchQ   string
	searchRes []database.GetPagePaginatedRow
	searchErr error
}

func (f *fakePageServicer) Create(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error) {
	if f.create != nil {
		return f.create(ctx, title, date)
	}

	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")

	return database.CreatePageRow{ID: id, Title: title}, nil
}

func (f *fakePageServicer) Delete(ctx context.Context, id string) error { return nil }
func (f *fakePageServicer) GetAll(ctx context.Context) ([]database.GetAllPagesRow, error) {
	return nil, nil
}

func (f *fakePageServicer) GetBackLinks(ctx context.Context, id string) ([]database.GetBacklinksRow, error) {
	return nil, nil
}

func (f *fakePageServicer) GetBacklinkPages(ctx context.Context, id string) ([]database.GetBacklinkPagesRow, error) {
	return nil, nil
}

func (f *fakePageServicer) GetById(ctx context.Context, id string) (database.GetPageByIDRow, error) {
	return database.GetPageByIDRow{}, nil
}

func (f *fakePageServicer) GetPagePaginated(ctx context.Context, page, limit int) ([]database.GetPagePaginatedRow, error) {
	return nil, nil
}

func (f *fakePageServicer) GetTotalPage(ctx context.Context) (int32, error) { return 0, nil }
func (f *fakePageServicer) Restore(ctx context.Context, id string) error    { return nil }
func (f *fakePageServicer) Update(ctx context.Context, title, id string, date *time.Time) (database.UpdatePageRow, error) {
	f.updated = true
	return database.UpdatePageRow{}, nil
}

func TestHandleCreatePage(t *testing.T) {
	fake := &fakePageServicer{}
	app := &App{service: &service.Services{Page: fake}}

	req := httptest.NewRequest(http.MethodPost, "/api/pages", bytes.NewBufferString(`{"title":"Alpha"}`))
	w := httptest.NewRecorder()

	app.handleCreatePage(w, req)

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
	if res.Title != "Alpha" {
		t.Fatalf("expected Alpha, got %q", res.Title)
	}
}

func TestHandleCreatePageBadBody(t *testing.T) {
	app := &App{service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pages", bytes.NewBufferString(`{"title":`))
	w := httptest.NewRecorder()

	app.handleCreatePage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreatePageServiceError(t *testing.T) {
	fake := &fakePageServicer{
		create: func(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error) {
			return database.CreatePageRow{}, service.ErrEmptyTitle
		},
	}
	app := &App{service: &service.Services{Page: fake}}

	req := httptest.NewRequest(http.MethodPost, "/api/pages", bytes.NewBufferString(`{"title":""}`))
	w := httptest.NewRecorder()

	app.handleCreatePage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if w.Body.String() != service.ErrEmptyTitle.Error()+"\n" {
		t.Fatalf("expected error body, got %q", w.Body.String())
	}
}

func TestHandleUpdatePage(t *testing.T) {
	fake := &fakePageServicer{}
	app := &App{service: &service.Services{Page: fake}}

	req := httptest.NewRequest(http.MethodPatch, "/api/pages/0197f1a0-0000-0000-0000-000000000001", bytes.NewBufferString(`{"title":"Renamed"}`))
	w := httptest.NewRecorder()

	app.handleUpdatePage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !fake.updated {
		t.Fatalf("expected fake service to be called")
	}
}

func TestHandlePages(t *testing.T) {
	app := &App{service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages", nil)
	w := httptest.NewRecorder()

	app.handlePages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.Page != 1 || res.Limit != 10 {
		t.Fatalf("expected defaults 1/10, got %d/%d", res.Page, res.Limit)
	}
}

func TestHandlePagesQueryParams(t *testing.T) {
	app := &App{service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages?page=3&limit=25", nil)
	w := httptest.NewRecorder()

	app.handlePages(w, req)

	var res struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.Page != 3 || res.Limit != 25 {
		t.Fatalf("expected 3/25, got %d/%d", res.Page, res.Limit)
	}
}

func TestHandlePagesFallsBackOnGarbageParams(t *testing.T) {
	app := &App{service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages?page=abc&limit=-5", nil)
	w := httptest.NewRecorder()

	app.handlePages(w, req)

	var res struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.Page != 1 || res.Limit != 10 {
		t.Fatalf("expected fallback 1/10, got %d/%d", res.Page, res.Limit)
	}
}

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
	app := &App{service: &service.Services{Transaction: &fakeTransactionServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/transactions/0197f1a0-0000-0000-0000-000000000001/restore", nil)
	w := httptest.NewRecorder()

	app.handleRestoreTransaction(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleRestoreTransactionError(t *testing.T) {
	app := &App{service: &service.Services{Transaction: &fakeTransactionServicer{restoreErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/transactions/x/restore", nil)
	w := httptest.NewRecorder()

	app.handleRestoreTransaction(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleRestorePocket(t *testing.T) {
	app := &App{service: &service.Services{Pocket: &fakePocketServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pockets/0197f1a0-0000-0000-0000-000000000001/restore", nil)
	w := httptest.NewRecorder()

	app.handleRestorePocket(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleRestorePocketError(t *testing.T) {
	app := &App{service: &service.Services{Pocket: &fakePocketServicer{restoreErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pockets/x/restore", nil)
	w := httptest.NewRecorder()

	app.handleRestorePocket(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleTrashFragment(t *testing.T) {
	pockets := []database.GetDeletedPocketsRow{deletedPocketRow("Cash2")}
	transactions := []database.GetDeletedTransactionsRow{deletedTransactionRow("Pindah")}
	app := &App{service: &service.Services{
		Pocket:      &fakePocketServicer{deleted: pockets},
		Transaction: &fakeTransactionServicer{deleted: transactions},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/trash", nil)
	w := httptest.NewRecorder()

	app.handleTrashFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Cash2") || !strings.Contains(body, "Pindah") {
		t.Fatalf("expected deleted pocket and transaction in body, got %s", body)
	}
}

func TestHandleTrashFragmentPocketError(t *testing.T) {
	app := &App{service: &service.Services{
		Pocket:      &fakePocketServicer{getDeletedErr: errors.New("boom")},
		Transaction: &fakeTransactionServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/trash", nil)
	w := httptest.NewRecorder()

	app.handleTrashFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleTrashFragmentTransactionError(t *testing.T) {
	app := &App{service: &service.Services{
		Pocket:      &fakePocketServicer{},
		Transaction: &fakeTransactionServicer{getDeletedErr: errors.New("boom")},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/trash", nil)
	w := httptest.NewRecorder()

	app.handleTrashFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleTransactionsFragmentWithFilters(t *testing.T) {
	fake := &fakeTransactionServicer{
		filtered: []database.GetTransactionsWithPocketNamesRow{deletedTransactionRowToRow()},
	}
	app := &App{service: &service.Services{
		Transaction: fake,
		Pocket:      &fakePocketServicer{},
		Taggable:    &fakeTaggableServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?pocket=0197f1a0-0000-0000-0000-000000000001&from=2026-08-01&to=2026-08-15", nil)
	w := httptest.NewRecorder()

	app.handleTransactionsFragment(w, req)

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
	app := &App{service: &service.Services{
		Transaction: fake,
		Pocket:      &fakePocketServicer{},
		Taggable:    &fakeTaggableServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions", nil)
	w := httptest.NewRecorder()

	app.handleTransactionsFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.filterArg != nil {
		t.Fatalf("expected GetWithPockets path without filters, got %v", fake.filterArg)
	}
}

func TestHandleTransactionsFragmentBadDate(t *testing.T) {
	app := &App{service: &service.Services{
		Transaction: &fakeTransactionServicer{},
		Pocket:      &fakePocketServicer{},
		Taggable:    &fakeTaggableServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?from=banana", nil)
	w := httptest.NewRecorder()

	app.handleTransactionsFragment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleTransactionsFragmentServiceError(t *testing.T) {
	app := &App{service: &service.Services{
		Transaction: &fakeTransactionServicer{filterErr: errors.New("boom")},
		Pocket:      &fakePocketServicer{},
		Taggable:    &fakeTaggableServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?from=2026-08-01", nil)
	w := httptest.NewRecorder()

	app.handleTransactionsFragment(w, req)

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

type fakeTaggableServicer struct {
	targetId      string
	kind          string
	tags          []database.GetTagsByTargetRow
	tagsErr       error
	pagesByTag    []database.GetPagePaginatedRow
	pagesByTagArg string
	pagesByTagErr error
}

func (f *fakeTaggableServicer) Attach(ctx context.Context, arg service.AttachTagArg) error {
	return nil
}

func (f *fakeTaggableServicer) Detach(ctx context.Context, arg service.DetachTagArg) error {
	return nil
}

func (f *fakeTaggableServicer) GetTagsByTargetId(ctx context.Context, id, kind string) ([]database.GetTagsByTargetRow, error) {
	f.targetId = id
	f.kind = kind
	return f.tags, f.tagsErr
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
	app := &App{service: &service.Services{
		Transaction: &fakeTransactionServicer{
			filtered: []database.GetTransactionsWithPocketNamesRow{taggedTransactionRow("Gaji")},
		},
		Pocket:   &fakePocketServicer{},
		Taggable: taggable,
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/transactions?from=2026-08-01", nil)
	w := httptest.NewRecorder()

	app.handleTransactionsFragment(w, req)

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
	app := &App{service: &service.Services{
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

	app.handleTransactionsFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func (f *fakePageServicer) Search(ctx context.Context, q string) ([]database.GetPagePaginatedRow, error) {
	f.searchQ = q
	return f.searchRes, f.searchErr
}

func TestHandlePagesSearch(t *testing.T) {
	fake := &fakePageServicer{
		searchRes: []database.GetPagePaginatedRow{{Title: "Makanan"}},
	}
	app := &App{service: &service.Services{Page: fake}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages?q=makan", nil)
	w := httptest.NewRecorder()

	app.handlePages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.searchQ != "makan" {
		t.Fatalf("expected query passthrough, got %q", fake.searchQ)
	}

	if !strings.Contains(w.Body.String(), "Makanan") {
		t.Fatalf("expected search result in body, got %s", w.Body.String())
	}
}

func TestHandlePagesSearchError(t *testing.T) {
	app := &App{service: &service.Services{Page: &fakePageServicer{searchErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages?q=makan", nil)
	w := httptest.NewRecorder()

	app.handlePages(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func (f *fakeTaggableServicer) GetPagesByTag(ctx context.Context, tagId string) ([]database.GetPagePaginatedRow, error) {
	f.pagesByTagArg = tagId
	return f.pagesByTag, f.pagesByTagErr
}

func TestHandleGetPagesByTag(t *testing.T) {
	fake := &fakeTaggableServicer{
		pagesByTag: []database.GetPagePaginatedRow{{Title: "Makanan"}},
	}
	app := &App{service: &service.Services{Taggable: fake}}

	req := httptest.NewRequest(http.MethodGet, "/api/tags/0197f1a0-0000-0000-0000-000000000001/pages", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	app.handleGetPagesByTag(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.pagesByTagArg != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected tag id passthrough, got %q", fake.pagesByTagArg)
	}

	if !strings.Contains(w.Body.String(), "Makanan") {
		t.Fatalf("expected page in body, got %s", w.Body.String())
	}
}

func TestHandleGetPagesByTagError(t *testing.T) {
	app := &App{service: &service.Services{Taggable: &fakeTaggableServicer{pagesByTagErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/tags/0197f1a0-0000-0000-0000-000000000001/pages", nil)
	w := httptest.NewRecorder()

	app.handleGetPagesByTag(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleTagPagesFragment(t *testing.T) {
	app := &App{service: &service.Services{Taggable: &fakeTaggableServicer{
		pagesByTag: []database.GetPagePaginatedRow{{Title: "Makanan"}},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/fin/tags/0197f1a0-0000-0000-0000-000000000001/pages?name=Makan", nil)
	w := httptest.NewRecorder()

	app.handleTagPagesFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Makanan") {
		t.Fatalf("expected page title in body, got %s", w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "Makan") {
		t.Fatalf("expected tag name in body, got %s", w.Body.String())
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

	app := &App{service: &service.Services{
		Pocket:      &fakePocketServicer{},
		Transaction: &fakeTransactionServicer{summary: summary},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/pockets", nil)
	w := httptest.NewRecorder()

	app.handlePocketsFragment(w, req)

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
	app := &App{service: &service.Services{
		Pocket:      &fakePocketServicer{},
		Transaction: &fakeTransactionServicer{summaryErr: errors.New("boom")},
	}}

	req := httptest.NewRequest(http.MethodGet, "/fin/pockets", nil)
	w := httptest.NewRecorder()

	app.handlePocketsFragment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

type fakeBlockServicer struct {
	blocks []database.GetBlocksByPageRow
	err    error
}

func (f *fakeBlockServicer) Create(ctx context.Context, args service.CreateBlockArgs) (database.CreateBlockRow, error) {
	return database.CreateBlockRow{}, nil
}

func (f *fakeBlockServicer) Delete(ctx context.Context, id string) error { return nil }

func (f *fakeBlockServicer) GetBlocksByPage(ctx context.Context, id string) ([]database.GetBlocksByPageRow, error) {
	return f.blocks, f.err
}

func (f *fakeBlockServicer) Restore(ctx context.Context, id string) error { return nil }

func (f *fakeBlockServicer) Update(ctx context.Context, args service.UpdateBlockArgs) (database.UpdateBlockRow, error) {
	return database.UpdateBlockRow{}, nil
}

func (f *fakeBlockServicer) Reorder(ctx context.Context, args []service.ReorderBlockArgs) error {
	return nil
}

func TestHandlePageFragmentFinance(t *testing.T) {
	balance := database.GetPocketBalancesRow{Name: "Wallet Utama", Type: "bank"}
	_ = balance.Balance.Scan("1500.50")

	pagetag := database.GetTagsByTargetRow{Name: "makan", Color: "red"}

	finBlock := func() database.GetBlocksByPageRow {
		var id pgtype.UUID
		_ = id.Scan("0197f1a0-0000-0000-0000-00000000000a")
		return database.GetBlocksByPageRow{ID: id, Type: "finance"}
	}

	baseServices := func() service.Services {
		return service.Services{
			Page: &fakePageServicer{},
			Block: &fakeBlockServicer{
				blocks: []database.GetBlocksByPageRow{finBlock()},
			},
			Taggable: &fakeTaggableServicer{tags: []database.GetTagsByTargetRow{pagetag}},
			Pocket:   &fakePocketServicer{balances: []database.GetPocketBalancesRow{balance}},
		}
	}

	t.Run("skips balance fetch without finance block", func(t *testing.T) {
		pocket := &fakePocketServicer{balances: []database.GetPocketBalancesRow{balance}}
		app := &App{service: &service.Services{
			Page:     &fakePageServicer{},
			Block:    &fakeBlockServicer{blocks: []database.GetBlocksByPageRow{{ID: finBlock().ID, Type: "text"}}},
			Taggable: &fakeTaggableServicer{},
			Pocket:   pocket,
		}}

		req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
		w := httptest.NewRecorder()

		app.handlePageFragment(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		if pocket.balanceCalls != 0 {
			t.Fatalf("expected no balance fetch, got %d calls", pocket.balanceCalls)
		}
	})

	t.Run("renders pocket balance for finance block", func(t *testing.T) {
		base := baseServices()
		app := &App{service: &base}

		req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
		w := httptest.NewRecorder()

		app.handlePageFragment(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "Wallet Utama") {
			t.Fatalf("expected pocket name in body, got %s", w.Body.String())
		}

		if !strings.Contains(w.Body.String(), "1500.50") {
			t.Fatalf("expected balance in body, got %s", w.Body.String())
		}
	})

	t.Run("500 when balance fetch fails", func(t *testing.T) {
		services := baseServices()
		services.Pocket = &fakePocketServicer{balancesErr: errors.New("boom")}

		app := &App{service: &services}

		req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
		w := httptest.NewRecorder()

		app.handlePageFragment(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w.Code)
		}
	})
}

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
	"github.com/jackc/pgx/v5/pgtype"
)

type fakePageServicer struct {
	create  func(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error)
	updated bool
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

func (f *fakeTransactionServicer) Restore(ctx context.Context, id string) error { return f.restoreErr }

func (f *fakeTransactionServicer) Update(ctx context.Context, arg service.UpdateTransactionArg) (database.UpdateTransactionRow, error) {
	return database.UpdateTransactionRow{}, nil
}

type fakePocketServicer struct {
	deleted       []database.GetDeletedPocketsRow
	getDeletedErr error
	restoreErr    error
}

func (f *fakePocketServicer) Create(ctx context.Context, name, kind string) (database.CreatePocketRow, error) {
	return database.CreatePocketRow{}, nil
}

func (f *fakePocketServicer) Delete(ctx context.Context, id string) error { return nil }

func (f *fakePocketServicer) GetAll(ctx context.Context) ([]database.GetAllPocketsRow, error) {
	return nil, nil
}

func (f *fakePocketServicer) GetBalances(ctx context.Context) ([]database.GetPocketBalancesRow, error) {
	return nil, nil
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

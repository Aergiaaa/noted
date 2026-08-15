package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

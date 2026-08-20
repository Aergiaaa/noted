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

type fakePageServicer struct {
	create       func(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error)
	updated      bool
	searchQ      string
	searchRes    []database.GetPagePaginatedRow
	searchErr    error
	byId         database.GetPageByIDRow
	byIdErr      error
	deleteErr    error
	restoreErr   error
	backlinks    []database.GetBacklinkPagesRow
	backlinksErr error
}

func (f *fakePageServicer) Create(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error) {
	if f.create != nil {
		return f.create(ctx, title, date)
	}

	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")

	return database.CreatePageRow{ID: id, Title: title}, nil
}

func (f *fakePageServicer) Delete(ctx context.Context, id string) error { return f.deleteErr }
func (f *fakePageServicer) GetAll(ctx context.Context) ([]database.GetAllPagesRow, error) {
	return nil, nil
}

func (f *fakePageServicer) GetBackLinks(ctx context.Context, id string) ([]database.GetBacklinksRow, error) {
	return nil, nil
}

func (f *fakePageServicer) GetBacklinkPages(ctx context.Context, id string) ([]database.GetBacklinkPagesRow, error) {
	return f.backlinks, f.backlinksErr
}

func (f *fakePageServicer) GetById(ctx context.Context, id string) (database.GetPageByIDRow, error) {
	return f.byId, f.byIdErr
}

func (f *fakePageServicer) GetPagePaginated(ctx context.Context, page, limit int) ([]database.GetPagePaginatedRow, error) {
	return nil, nil
}

func (f *fakePageServicer) GetTotalPage(ctx context.Context) (int32, error) { return 0, nil }
func (f *fakePageServicer) Restore(ctx context.Context, id string) error    { return f.restoreErr }
func (f *fakePageServicer) Update(ctx context.Context, title, id string, date *time.Time) (database.UpdatePageRow, error) {
	f.updated = true
	return database.UpdatePageRow{}, nil
}

func TestHandleCreatePage(t *testing.T) {
	fake := &fakePageServicer{}
	h := Handler{Service: &service.Services{Page: fake}}

	req := httptest.NewRequest(http.MethodPost, "/api/pages", bytes.NewBufferString(`{"title":"Alpha"}`))
	w := httptest.NewRecorder()

	h.CreatePage(w, req)

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
	h := Handler{Service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/pages", bytes.NewBufferString(`{"title":`))
	w := httptest.NewRecorder()

	h.CreatePage(w, req)

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
	h := Handler{Service: &service.Services{Page: fake}}

	req := httptest.NewRequest(http.MethodPost, "/api/pages", bytes.NewBufferString(`{"title":""}`))
	w := httptest.NewRecorder()

	h.CreatePage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if w.Body.String() != service.ErrEmptyTitle.Error()+"\n" {
		t.Fatalf("expected error body, got %q", w.Body.String())
	}
}

func TestHandleUpdatePage(t *testing.T) {
	fake := &fakePageServicer{}
	h := Handler{Service: &service.Services{Page: fake}}

	req := httptest.NewRequest(http.MethodPatch, "/api/pages/0197f1a0-0000-0000-0000-000000000001", bytes.NewBufferString(`{"title":"Renamed"}`))
	w := httptest.NewRecorder()

	h.UpdatePage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !fake.updated {
		t.Fatalf("expected fake service to be called")
	}
}

func TestHandlePages(t *testing.T) {
	h := Handler{Service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages", nil)
	w := httptest.NewRecorder()

	h.Pages(w, req)

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
	h := Handler{Service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages?page=3&limit=25", nil)
	w := httptest.NewRecorder()

	h.Pages(w, req)

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
	h := Handler{Service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages?page=abc&limit=-5", nil)
	w := httptest.NewRecorder()

	h.Pages(w, req)

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

func (f *fakePageServicer) Search(ctx context.Context, q string) ([]database.GetPagePaginatedRow, error) {
	f.searchQ = q
	return f.searchRes, f.searchErr
}

func TestHandlePagesSearch(t *testing.T) {
	fake := &fakePageServicer{
		searchRes: []database.GetPagePaginatedRow{{Title: "Makanan"}},
	}
	h := Handler{Service: &service.Services{Page: fake}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages?q=makan", nil)
	w := httptest.NewRecorder()

	h.Pages(w, req)

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
	h := Handler{Service: &service.Services{Page: &fakePageServicer{searchErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/pages?q=makan", nil)
	w := httptest.NewRecorder()

	h.Pages(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandlePageFragmentDeleteButton(t *testing.T) {
	h := Handler{Service: &service.Services{
		Page:        &fakePageServicer{},
		Block:       &fakeBlockServicer{},
		Taggable:    &fakeTaggableServicer{},
		Pocket:      &fakePocketServicer{},
		Transaction: &fakeTransactionServicer{},
	}}

	req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
	w := httptest.NewRecorder()

	h.RenderPageFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), `title="delete page"`) {
		t.Fatalf("expected delete page button in body, got %s", w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "askDeletePage(&#39;") {
		t.Fatalf("expected confirm dialog click in body, got %s", w.Body.String())
	}

	if strings.Contains(w.Body.String(), "deletePage(&#39;") {
		t.Fatalf("expected no direct page delete click in body, got %s", w.Body.String())
	}
}

type fakeBlockServicer struct {
	blocks      []database.GetBlocksByPageRow
	err         error
	createArgs  service.CreateBlockArgs
	createRow   database.CreateBlockRow
	createErr   error
	updateArgs  service.UpdateBlockArgs
	updateRow   database.UpdateBlockRow
	updateErr   error
	deleteErr   error
	restoreErr  error
	reorderArgs []service.ReorderBlockArgs
	reorderErr  error
}

func (f *fakeBlockServicer) Create(ctx context.Context, args service.CreateBlockArgs) (database.CreateBlockRow, error) {
	f.createArgs = args
	return f.createRow, f.createErr
}

func (f *fakeBlockServicer) Delete(ctx context.Context, id string) error { return f.deleteErr }

func (f *fakeBlockServicer) GetBlocksByPage(ctx context.Context, id string) ([]database.GetBlocksByPageRow, error) {
	return f.blocks, f.err
}

func (f *fakeBlockServicer) Restore(ctx context.Context, id string) error { return f.restoreErr }

func (f *fakeBlockServicer) Update(ctx context.Context, args service.UpdateBlockArgs) (database.UpdateBlockRow, error) {
	f.updateArgs = args
	return f.updateRow, f.updateErr
}

func (f *fakeBlockServicer) Reorder(ctx context.Context, args []service.ReorderBlockArgs) error {
	f.reorderArgs = args
	return f.reorderErr
}

func TestParseFinancePocket(t *testing.T) {
	for name, tc := range map[string]struct {
		content []byte
		want    string
	}{
		"pocket set": {
			content: []byte(`{"finance":{"pocket":"0197f1a0-0000-0000-0000-00000000000b"}}`),
			want:    "0197f1a0-0000-0000-0000-00000000000b",
		},
		"no pocket":   {content: []byte(`{"finance":{"pocket":""}}`), want: ""},
		"not finance": {content: []byte(`{"text":"hi"}`), want: ""},
		"malformed":   {content: []byte(`nope`), want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := parseFinancePocket(tc.content); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
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

	finBlockWithPocket := func() database.GetBlocksByPageRow {
		var id pgtype.UUID
		_ = id.Scan("0197f1a0-0000-0000-0000-00000000000a")
		return database.GetBlocksByPageRow{
			ID:      id,
			Type:    "finance",
			Content: []byte(`{"finance":{"pocket":"0197f1a0-0000-0000-0000-00000000000b"}}`),
		}
	}

	txn := database.GetTransactionsWithPocketNamesRow{Title: "Makan siang", Type: "expense"}
	_ = txn.Amount.Scan("25.00")

	baseServices := func() service.Services {
		return service.Services{
			Page: &fakePageServicer{},
			Block: &fakeBlockServicer{
				blocks: []database.GetBlocksByPageRow{finBlockWithPocket()},
			},
			Taggable:    &fakeTaggableServicer{tags: []database.GetTagsByTargetRow{pagetag}},
			Pocket:      &fakePocketServicer{balances: []database.GetPocketBalancesRow{balance}},
			Transaction: &fakeTransactionServicer{filtered: []database.GetTransactionsWithPocketNamesRow{txn}},
		}
	}

	t.Run("skips balance fetch without finance block", func(t *testing.T) {
		pocket := &fakePocketServicer{balances: []database.GetPocketBalancesRow{balance}}
		h := Handler{Service: &service.Services{
			Page:     &fakePageServicer{},
			Block:    &fakeBlockServicer{blocks: []database.GetBlocksByPageRow{{ID: finBlock().ID, Type: "text"}}},
			Taggable: &fakeTaggableServicer{},
			Pocket:   pocket,
		}}

		req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
		w := httptest.NewRecorder()

		h.RenderPageFragment(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		if pocket.balanceCalls != 0 {
			t.Fatalf("expected no balance fetch, got %d calls", pocket.balanceCalls)
		}
	})

	t.Run("renders pocket balance for finance block", func(t *testing.T) {
		base := baseServices()
		h := Handler{Service: &base}

		req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
		w := httptest.NewRecorder()

		h.RenderPageFragment(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "Wallet Utama") {
			t.Fatalf("expected pocket name in body, got %s", w.Body.String())
		}

		if !strings.Contains(w.Body.String(), "1500.50") {
			t.Fatalf("expected balance in body, got %s", w.Body.String())
		}

		if !strings.Contains(w.Body.String(), "Makan siang") {
			t.Fatalf("expected transaction title in body, got %s", w.Body.String())
		}

		if !strings.Contains(w.Body.String(), "-25.00") {
			t.Fatalf("expected signed amount in body, got %s", w.Body.String())
		}
	})

	t.Run("skips transactions fetch without finance block", func(t *testing.T) {
		txm := &fakeTransactionServicer{}
		h := Handler{Service: &service.Services{
			Page:        &fakePageServicer{},
			Block:       &fakeBlockServicer{blocks: []database.GetBlocksByPageRow{{ID: finBlock().ID, Type: "text"}}},
			Taggable:    &fakeTaggableServicer{},
			Pocket:      &fakePocketServicer{},
			Transaction: txm,
		}}

		req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
		w := httptest.NewRecorder()

		h.RenderPageFragment(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		if txm.filterArg != nil {
			t.Fatalf("expected no transaction fetch, got %+v", txm.filterArg)
		}
	})

	t.Run("500 when transactions fetch fails", func(t *testing.T) {
		services := baseServices()
		services.Transaction = &fakeTransactionServicer{filterErr: errors.New("boom")}

		h := Handler{Service: &services}

		req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
		w := httptest.NewRecorder()

		h.RenderPageFragment(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w.Code)
		}
	})

	t.Run("500 when balance fetch fails", func(t *testing.T) {
		services := baseServices()
		services.Pocket = &fakePocketServicer{balancesErr: errors.New("boom")}

		h := Handler{Service: &services}

		req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/fragment", nil)
		w := httptest.NewRecorder()

		h.RenderPageFragment(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w.Code)
		}
	})
}

func TestHandleGetPage(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	h := Handler{Service: &service.Services{Page: &fakePageServicer{
		byId: database.GetPageByIDRow{ID: id, Title: "Alpha"},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetPage(w, req)

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
		t.Fatalf("expected title, got %q", res.Title)
	}
}

func TestHandleGetPageError(t *testing.T) {
	h := Handler{Service: &service.Services{Page: &fakePageServicer{byIdErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/pages/x", nil)
	w := httptest.NewRecorder()

	h.GetPage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleDeletePage(t *testing.T) {
	h := Handler{Service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodDelete, "/pages/0197f1a0-0000-0000-0000-000000000001", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeletePage(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleDeletePageError(t *testing.T) {
	h := Handler{Service: &service.Services{Page: &fakePageServicer{deleteErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodDelete, "/pages/x", nil)
	w := httptest.NewRecorder()

	h.DeletePage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleRestorePage(t *testing.T) {
	h := Handler{Service: &service.Services{Page: &fakePageServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/pages/0197f1a0-0000-0000-0000-000000000001/restore", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.RestorePage(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleRestorePageError(t *testing.T) {
	h := Handler{Service: &service.Services{Page: &fakePageServicer{restoreErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/pages/x/restore", nil)
	w := httptest.NewRecorder()

	h.RestorePage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleGetPageBlocks(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{
		blocks: []database.GetBlocksByPageRow{{ID: id, Type: "text", Content: []byte(`{"text":"hi"}`)}},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/blocks", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetPageBlocks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		Blocks []database.GetBlocksByPageRow `json:"blocks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(res.Blocks) != 1 || res.Blocks[0].Type != "text" {
		t.Fatalf("expected block in response, got %+v", res.Blocks)
	}
}

func TestHandleGetPageBlocksError(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{err: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/pages/x/blocks", nil)
	w := httptest.NewRecorder()

	h.GetPageBlocks(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleGetBacklinks(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	h := Handler{Service: &service.Services{Page: &fakePageServicer{
		backlinks: []database.GetBacklinkPagesRow{{ID: id, Title: "Linked Page"}},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/pages/0197f1a0-0000-0000-0000-000000000001/backlinks", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetBacklinks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		Backlinks []database.GetBacklinkPagesRow `json:"backlinks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(res.Backlinks) != 1 || res.Backlinks[0].Title != "Linked Page" {
		t.Fatalf("expected backlink in response, got %+v", res.Backlinks)
	}
}

func TestHandleGetBacklinksError(t *testing.T) {
	h := Handler{Service: &service.Services{Page: &fakePageServicer{backlinksErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/pages/x/backlinks", nil)
	w := httptest.NewRecorder()

	h.GetBacklinks(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

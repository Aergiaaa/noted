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
	"testing"
)

type fakeEdgeServicer struct {
	edges      []database.GetEdgesRow
	edgesErr   error
	createArg  service.CreateEdgeArg
	createErr  error
	deleteArg  service.DeleteEdgeArg
	deleteErr  error
	syncPageID string
	syncTitles []string
	syncErr    error
}

func (f *fakeEdgeServicer) Create(ctx context.Context, arg service.CreateEdgeArg) error {
	f.createArg = arg
	return f.createErr
}

func (f *fakeEdgeServicer) Delete(ctx context.Context, arg service.DeleteEdgeArg) error {
	f.deleteArg = arg
	return f.deleteErr
}

func (f *fakeEdgeServicer) GetAll(ctx context.Context) ([]database.GetEdgesRow, error) {
	return f.edges, f.edgesErr
}

func (f *fakeEdgeServicer) SyncWikiLinks(ctx context.Context, pageId string, titles []string) error {
	f.syncPageID = pageId
	f.syncTitles = titles
	return f.syncErr
}

func TestHandleGetEdges(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	h := Handler{Service: &service.Services{Edge: &fakeEdgeServicer{
		edges: []database.GetEdgesRow{{
			FromID:   id,
			FromType: "page",
			ToID:     id,
			ToType:   "page",
			LinkType: "wiki-link",
		}},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/edges", nil)
	w := httptest.NewRecorder()

	h.GetEdges(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		Edges []database.GetEdgesRow `json:"edges"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(res.Edges) != 1 || res.Edges[0].LinkType != "wiki-link" {
		t.Fatalf("expected edge in response, got %+v", res.Edges)
	}
}

func TestHandleGetEdgesError(t *testing.T) {
	h := Handler{Service: &service.Services{Edge: &fakeEdgeServicer{edgesErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/edges", nil)
	w := httptest.NewRecorder()

	h.GetEdges(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleCreateEdge(t *testing.T) {
	fake := &fakeEdgeServicer{}
	h := Handler{Service: &service.Services{Edge: fake}}

	req := httptest.NewRequest(http.MethodPost, "/api/edges", bytes.NewBufferString(`{"from_id":"0197f1a0-0000-0000-0000-000000000001","from_type":"page","to_id":"0197f1a0-0000-0000-0000-000000000002","to_type":"page","link_type":"wiki-link"}`))
	w := httptest.NewRecorder()

	h.CreateEdge(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	if fake.createArg.FromID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected from id passthrough, got %q", fake.createArg.FromID)
	}

	if fake.createArg.LinkType != "wiki-link" {
		t.Fatalf("expected link type passthrough, got %q", fake.createArg.LinkType)
	}
}

func TestHandleCreateEdgeBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Edge: &fakeEdgeServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/edges", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.CreateEdge(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateEdgeServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Edge: &fakeEdgeServicer{createErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/edges", bytes.NewBufferString(`{"from_id":"x"}`))
	w := httptest.NewRecorder()

	h.CreateEdge(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleDeleteEdge(t *testing.T) {
	fake := &fakeEdgeServicer{}
	h := Handler{Service: &service.Services{Edge: fake}}

	req := httptest.NewRequest(http.MethodDelete, "/api/edges", bytes.NewBufferString(`{"from_id":"0197f1a0-0000-0000-0000-000000000001","to_id":"0197f1a0-0000-0000-0000-000000000002","link_type":"wiki-link"}`))
	w := httptest.NewRecorder()

	h.DeleteEdge(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	if fake.deleteArg.FromId != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected from id passthrough, got %q", fake.deleteArg.FromId)
	}

	if fake.deleteArg.ToId != "0197f1a0-0000-0000-0000-000000000002" {
		t.Fatalf("expected to id passthrough, got %q", fake.deleteArg.ToId)
	}
}

func TestHandleDeleteEdgeBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Edge: &fakeEdgeServicer{}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/edges", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.DeleteEdge(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleDeleteEdgeServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Edge: &fakeEdgeServicer{deleteErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/edges", bytes.NewBufferString(`{"from_id":"x"}`))
	w := httptest.NewRecorder()

	h.DeleteEdge(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleSyncWikiLinks(t *testing.T) {
	fake := &fakeEdgeServicer{}
	h := Handler{Service: &service.Services{Edge: fake}}

	req := httptest.NewRequest(http.MethodPost, "/pages/0197f1a0-0000-0000-0000-000000000001/wiki-links", bytes.NewBufferString(`{"titles":["Satu","Dua"]}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.SyncWikiLinks(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	if fake.syncPageID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected page id passthrough, got %q", fake.syncPageID)
	}

	if len(fake.syncTitles) != 2 || fake.syncTitles[0] != "Satu" {
		t.Fatalf("expected titles passthrough, got %v", fake.syncTitles)
	}
}

func TestHandleSyncWikiLinksBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Edge: &fakeEdgeServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/pages/x/wiki-links", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.SyncWikiLinks(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSyncWikiLinksServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Edge: &fakeEdgeServicer{syncErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/pages/x/wiki-links", bytes.NewBufferString(`{"titles":[]}`))
	w := httptest.NewRecorder()

	h.SyncWikiLinks(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

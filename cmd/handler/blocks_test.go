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

func TestHandleCreateBlock(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	fake := &fakeBlockServicer{
		createRow: database.CreateBlockRow{ID: id, Type: "text", Content: []byte(`{"text":"hi"}`), Order: 0},
	}
	h := Handler{Service: &service.Services{Block: fake}}

	req := httptest.NewRequest(http.MethodPost, "/pages/0197f1a0-0000-0000-0000-00000000000a/blocks", bytes.NewBufferString(`{"type":"text","content":{"text":"hi"},"parent_id":""}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-00000000000a")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.CreateBlock(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.createArgs.PageID != "0197f1a0-0000-0000-0000-00000000000a" {
		t.Fatalf("expected page id passthrough, got %q", fake.createArgs.PageID)
	}

	if fake.createArgs.Type != "text" {
		t.Fatalf("expected type passthrough, got %q", fake.createArgs.Type)
	}

	var res struct {
		ID      string          `json:"id"`
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
		Order   int32           `json:"order"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.ID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected id, got %q", res.ID)
	}
	if res.Type != "text" {
		t.Fatalf("expected text type, got %q", res.Type)
	}
	if string(res.Content) != `{"text":"hi"}` {
		t.Fatalf("expected content, got %s", string(res.Content))
	}
}

func TestHandleCreateBlockBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/pages/x/blocks", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.CreateBlock(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateBlockServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{createErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/pages/x/blocks", bytes.NewBufferString(`{"type":"text"}`))
	w := httptest.NewRecorder()

	h.CreateBlock(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleUpdateBlock(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	fake := &fakeBlockServicer{
		updateRow: database.UpdateBlockRow{ID: id, Type: "text", Content: []byte(`{"text":"bye"}`)},
	}
	h := Handler{Service: &service.Services{Block: fake}}

	req := httptest.NewRequest(http.MethodPatch, "/blocks/0197f1a0-0000-0000-0000-000000000001", bytes.NewBufferString(`{"type":"text","content":{"text":"bye"}}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateBlock(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if fake.updateArgs.Id != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected block id passthrough, got %q", fake.updateArgs.Id)
	}

	var res struct {
		ID      string          `json:"id"`
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.ID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected id, got %q", res.ID)
	}
	if string(res.Content) != `{"text":"bye"}` {
		t.Fatalf("expected content, got %s", string(res.Content))
	}
}

func TestHandleUpdateBlockBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{}}}

	req := httptest.NewRequest(http.MethodPatch, "/blocks/x", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.UpdateBlock(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateBlockServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{updateErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPatch, "/blocks/x", bytes.NewBufferString(`{"type":"text"}`))
	w := httptest.NewRecorder()

	h.UpdateBlock(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleDeleteBlock(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{}}}

	req := httptest.NewRequest(http.MethodDelete, "/blocks/0197f1a0-0000-0000-0000-000000000001", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteBlock(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleDeleteBlockError(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{deleteErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodDelete, "/blocks/x", nil)
	w := httptest.NewRecorder()

	h.DeleteBlock(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleRestoreBlock(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/blocks/0197f1a0-0000-0000-0000-000000000001/restore", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.RestoreBlock(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleRestoreBlockError(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{restoreErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/blocks/x/restore", nil)
	w := httptest.NewRecorder()

	h.RestoreBlock(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleReorderBlocks(t *testing.T) {
	fake := &fakeBlockServicer{}
	h := Handler{Service: &service.Services{Block: fake}}

	req := httptest.NewRequest(http.MethodPut, "/blocks/reorder", bytes.NewBufferString(`[{"id":"0197f1a0-0000-0000-0000-000000000001","order":2}]`))
	w := httptest.NewRecorder()

	h.ReorderBlocks(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	if len(fake.reorderArgs) != 1 {
		t.Fatalf("expected one reorder arg, got %d", len(fake.reorderArgs))
	}

	if fake.reorderArgs[0].ID != "0197f1a0-0000-0000-0000-000000000001" || fake.reorderArgs[0].Order != 2 {
		t.Fatalf("expected id and order passthrough, got %+v", fake.reorderArgs[0])
	}
}

func TestHandleReorderBlocksBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{}}}

	req := httptest.NewRequest(http.MethodPut, "/blocks/reorder", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.ReorderBlocks(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleReorderBlocksServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Block: &fakeBlockServicer{reorderErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPut, "/blocks/reorder", bytes.NewBufferString(`[]`))
	w := httptest.NewRecorder()

	h.ReorderBlocks(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

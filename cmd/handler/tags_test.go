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
)

type fakeTaggableServicer struct {
	targetId      string
	kind          string
	tags          []database.GetTagsByTargetRow
	tagsErr       error
	pagesByTag    []database.GetPagePaginatedRow
	pagesByTagArg string
	pagesByTagErr error
	attachArg     service.AttachTagArg
	attachErr     error
	detachArg     service.DetachTagArg
	detachErr     error
}

func (f *fakeTaggableServicer) Attach(ctx context.Context, arg service.AttachTagArg) error {
	f.attachArg = arg
	return f.attachErr
}

func (f *fakeTaggableServicer) Detach(ctx context.Context, arg service.DetachTagArg) error {
	f.detachArg = arg
	return f.detachErr
}

func (f *fakeTaggableServicer) GetTagsByTargetId(ctx context.Context, id, kind string) ([]database.GetTagsByTargetRow, error) {
	f.targetId = id
	f.kind = kind
	return f.tags, f.tagsErr
}

func (f *fakeTaggableServicer) GetPagesByTag(ctx context.Context, tagId string) ([]database.GetPagePaginatedRow, error) {
	f.pagesByTagArg = tagId
	return f.pagesByTag, f.pagesByTagErr
}

func TestHandleGetPagesByTag(t *testing.T) {
	fake := &fakeTaggableServicer{
		pagesByTag: []database.GetPagePaginatedRow{{Title: "Makanan"}},
	}
	h := Handler{Service: &service.Services{Taggable: fake}}

	req := httptest.NewRequest(http.MethodGet, "/api/tags/0197f1a0-0000-0000-0000-000000000001/pages", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetPagesByTag(w, req)

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
	h := Handler{Service: &service.Services{Taggable: &fakeTaggableServicer{pagesByTagErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/tags/0197f1a0-0000-0000-0000-000000000001/pages", nil)
	w := httptest.NewRecorder()

	h.GetPagesByTag(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleTagPagesFragment(t *testing.T) {
	h := Handler{Service: &service.Services{Taggable: &fakeTaggableServicer{
		pagesByTag: []database.GetPagePaginatedRow{{Title: "Makanan"}},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/fin/tags/0197f1a0-0000-0000-0000-000000000001/pages?name=Makan", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.TagPagesFragment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Makanan") {
		t.Fatalf("expected page title in body, got %s", w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "Makan") {
		t.Fatalf("expected tag name in body, got %s", w.Body.String())
	}

	if !strings.Contains(w.Body.String(), `title="delete tag"`) {
		t.Fatalf("expected delete tag button in body, got %s", w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "askDeleteTag(&#39;0197f1a0-0000-0000-0000-000000000001&#39;)") {
		t.Fatalf("expected delete tag click in body, got %s", w.Body.String())
	}
}

type fakeTagServicer struct {
	all        []database.GetAllTagsRow
	allErr     error
	createRow  database.CreateTagRow
	createErr  error
	updateRow  database.UpdateTagRow
	updateErr  error
	deleteErr  error
	restoreErr error
}

func (f *fakeTagServicer) Create(ctx context.Context, name, color string) (database.CreateTagRow, error) {
	return f.createRow, f.createErr
}

func (f *fakeTagServicer) Delete(ctx context.Context, id string) error { return f.deleteErr }

func (f *fakeTagServicer) GetAll(ctx context.Context) ([]database.GetAllTagsRow, error) {
	return f.all, f.allErr
}

func (f *fakeTagServicer) Restore(ctx context.Context, id string) error { return f.restoreErr }

func (f *fakeTagServicer) Update(ctx context.Context, name, color, id string) (database.UpdateTagRow, error) {
	return f.updateRow, f.updateErr
}

func TestHandleGetTags(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{
		all: []database.GetAllTagsRow{{ID: id, Name: "makan", Color: "red"}},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()

	h.GetTags(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		Tags []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(res.Tags) != 1 || res.Tags[0].Name != "makan" {
		t.Fatalf("expected tag in response, got %+v", res.Tags)
	}

	if res.Tags[0].ID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected tag id, got %q", res.Tags[0].ID)
	}
}

func TestHandleGetTagsError(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{allErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()

	h.GetTags(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleCreateTag(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{
		createRow: database.CreateTagRow{ID: id, Name: "makan", Color: "red"},
	}}}

	req := httptest.NewRequest(http.MethodPost, "/api/tags", bytes.NewBufferString(`{"name":"makan","color":"red"}`))
	w := httptest.NewRecorder()

	h.CreateTag(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.ID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected id, got %q", res.ID)
	}
	if res.Name != "makan" || res.Color != "red" {
		t.Fatalf("expected name and color, got %s/%s", res.Name, res.Color)
	}
}

func TestHandleCreateTagBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/tags", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.CreateTag(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateTagServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{createErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/tags", bytes.NewBufferString(`{"name":"makan"}`))
	w := httptest.NewRecorder()

	h.CreateTag(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleUpdateTag(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("0197f1a0-0000-0000-0000-000000000001")
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{
		updateRow: database.UpdateTagRow{ID: id, Name: "makan", Color: "blue"},
	}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/tags/0197f1a0-0000-0000-0000-000000000001", bytes.NewBufferString(`{"name":"makan","color":"blue"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateTag(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if res.Color != "blue" {
		t.Fatalf("expected color, got %q", res.Color)
	}
}

func TestHandleUpdateTagBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/tags/x", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.UpdateTag(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateTagServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{updateErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/tags/x", bytes.NewBufferString(`{"name":"makan"}`))
	w := httptest.NewRecorder()

	h.UpdateTag(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleDeleteTag(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/tags/0197f1a0-0000-0000-0000-000000000001", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteTag(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleDeleteTagError(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{deleteErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/tags/x", nil)
	w := httptest.NewRecorder()

	h.DeleteTag(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleRestoreTag(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/tags/0197f1a0-0000-0000-0000-000000000001/restore", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.RestoreTag(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleRestoreTagError(t *testing.T) {
	h := Handler{Service: &service.Services{Tag: &fakeTagServicer{restoreErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/tags/x/restore", nil)
	w := httptest.NewRecorder()

	h.RestoreTag(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleAttachTag(t *testing.T) {
	fake := &fakeTaggableServicer{}
	h := Handler{Service: &service.Services{Taggable: fake}}

	req := httptest.NewRequest(http.MethodPost, "/api/tags/0197f1a0-0000-0000-0000-000000000001/attach", bytes.NewBufferString(`{"target_id":"0197f1a0-0000-0000-0000-000000000002","target_type":"page"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.AttachTag(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	if fake.attachArg.TagID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected tag id passthrough, got %q", fake.attachArg.TagID)
	}

	if fake.attachArg.TargetID != "0197f1a0-0000-0000-0000-000000000002" || fake.attachArg.TargetType != "page" {
		t.Fatalf("expected target passthrough, got %+v", fake.attachArg)
	}
}

func TestHandleAttachTagBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Taggable: &fakeTaggableServicer{}}}

	req := httptest.NewRequest(http.MethodPost, "/api/tags/x/attach", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.AttachTag(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleAttachTagServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Taggable: &fakeTaggableServicer{attachErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodPost, "/api/tags/x/attach", bytes.NewBufferString(`{"target_id":"x"}`))
	w := httptest.NewRecorder()

	h.AttachTag(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleDetachTag(t *testing.T) {
	fake := &fakeTaggableServicer{}
	h := Handler{Service: &service.Services{Taggable: fake}}

	req := httptest.NewRequest(http.MethodDelete, "/api/tags/0197f1a0-0000-0000-0000-000000000001/detach", bytes.NewBufferString(`{"target_id":"0197f1a0-0000-0000-0000-000000000002","target_type":"page"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "0197f1a0-0000-0000-0000-000000000001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DetachTag(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	if fake.detachArg.TagID != "0197f1a0-0000-0000-0000-000000000001" {
		t.Fatalf("expected tag id passthrough, got %q", fake.detachArg.TagID)
	}

	if fake.detachArg.TargetID != "0197f1a0-0000-0000-0000-000000000002" || fake.detachArg.TargetType != "page" {
		t.Fatalf("expected target passthrough, got %+v", fake.detachArg)
	}
}

func TestHandleDetachTagBadBody(t *testing.T) {
	h := Handler{Service: &service.Services{Taggable: &fakeTaggableServicer{}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/tags/x/detach", bytes.NewBufferString(`{`))
	w := httptest.NewRecorder()

	h.DetachTag(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleDetachTagServiceError(t *testing.T) {
	h := Handler{Service: &service.Services{Taggable: &fakeTaggableServicer{detachErr: errors.New("boom")}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/tags/x/detach", bytes.NewBufferString(`{"target_id":"x"}`))
	w := httptest.NewRecorder()

	h.DetachTag(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

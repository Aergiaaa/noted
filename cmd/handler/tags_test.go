package handler

import (
	"context"
	"errors"
	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/service"
	"github.com/go-chi/chi/v5"
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

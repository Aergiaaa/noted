package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/service"
	"github.com/Aergiaaa/noted/ui/modules"
	"github.com/go-chi/chi/v5"
)

func (app *App) handleNOP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("NOP"))
}

func (a *App) handlePages(w http.ResponseWriter, r *http.Request) {
	pageQuery := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageQuery)
	if err != nil || page < 1 {
		page = 1
	}

	limitQuery := r.URL.Query().Get("limit")
	limit, err := strconv.Atoi(limitQuery)
	if err != nil || limit < 1 {
		limit = 10
	}

	pages, err := a.service.Page.GetPagePaginated(r.Context(), page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	totalPages, err := a.service.Page.GetTotalPage(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Pages      []database.GetPagePaginatedRow `json:"pages"`
		Page       int                            `json:"page"`
		Limit      int                            `json:"limit"`
		TotalPages int32                          `json:"total_pages"`
	}

	res := response{
		Pages:      pages,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handlePageFragment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	page, err := a.service.Page.GetById(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	blocks, err := a.service.Block.GetBlocksByPage(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tags, err := a.service.Taggable.GetTagsByTargetId(r.Context(), id, "page")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	backlinks, err := a.service.Page.GetBacklinkPages(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	modules.PageView(page.Title, page.ID.String(), blocks, tags, backlinks).Render(r.Context(), w)
}

func (a *App) handleCreatePage(w http.ResponseWriter, r *http.Request) {
	input := struct {
		Title string `json:"title"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page, err := a.service.Page.Create(r.Context(), input.Title, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}

	res := response{
		ID:    page.ID.String(),
		Title: page.Title,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleCreateBlock(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	input := struct {
		Type     string          `json:"type"`
		Content  json.RawMessage `json:"content"`
		ParentID string          `json:"parent_id"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	block, err := a.service.Block.Create(r.Context(), service.CreateBlockArgs{
		PageID:   pageId,
		ParentID: input.ParentID,
		Type:     input.Type,
		Content:  input.Content,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID      string          `json:"id"`
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
		Order   int32           `json:"order"`
	}

	res := response{
		ID:      block.ID.String(),
		Type:    block.Type,
		Content: json.RawMessage(block.Content),
		Order:   block.Order,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleUpdateBlock(w http.ResponseWriter, r *http.Request) {
	blockId := chi.URLParam(r, "id")

	input := struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	block, err := a.service.Block.Update(r.Context(), service.UpdateBlockArgs{
		Id:      blockId,
		Type:    input.Type,
		Content: input.Content,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID      string          `json:"id"`
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}

	res := response{
		ID:      block.ID.String(),
		Type:    block.Type,
		Content: json.RawMessage(block.Content),
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleDeleteBlock(w http.ResponseWriter, r *http.Request) {
	blockId := chi.URLParam(r, "id")

	if err := a.service.Block.Delete(r.Context(), blockId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRestoreBlock(w http.ResponseWriter, r *http.Request) {
	blockId := chi.URLParam(r, "id")

	if err := a.service.Block.Restore(r.Context(), blockId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleGetPage(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	page, err := a.service.Page.GetById(r.Context(), pageId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}

	res := response{
		ID:    page.ID.String(),
		Title: page.Title,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleUpdatePage(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	input := struct {
		Title string `json:"title"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page, err := a.service.Page.Update(r.Context(), input.Title, pageId, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}

	res := response{
		ID:    page.ID.String(),
		Title: page.Title,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleDeletePage(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	if err := a.service.Page.Delete(r.Context(), pageId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRestorePage(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	if err := a.service.Page.Restore(r.Context(), pageId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleGetPageBlocks(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	blocks, err := a.service.Block.GetBlocksByPage(r.Context(), pageId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Blocks []database.GetBlocksByPageRow `json:"blocks"`
	}

	res := response{
		Blocks: blocks,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleGetBacklinks(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	links, err := a.service.Page.GetBacklinkPages(r.Context(), pageId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Backlinks []database.GetBacklinkPagesRow `json:"backlinks"`
	}

	res := response{
		Backlinks: links,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleGetTags(w http.ResponseWriter, r *http.Request) {
	tags, err := a.service.Tag.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Tags []database.GetAllTagsRow `json:"tags"`
	}

	res := response{
		Tags: tags,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	input := struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tag, err := a.service.Tag.Create(r.Context(), input.Name, input.Color)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}

	res := response{
		ID:    tag.ID.String(),
		Name:  tag.Name,
		Color: tag.Color,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleUpdateTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	input := struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tag, err := a.service.Tag.Update(r.Context(), input.Name, input.Color, tagId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}

	res := response{
		ID:    tag.ID.String(),
		Name:  tag.Name,
		Color: tag.Color,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	if err := a.service.Tag.Delete(r.Context(), tagId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRestoreTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	if err := a.service.Tag.Restore(r.Context(), tagId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAttachTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	input := struct {
		TargetID   string `json:"target_id"`
		TargetType string `json:"target_type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.service.Taggable.Attach(r.Context(), service.AttachTagArg{
		TagID:      tagId,
		TargetID:   input.TargetID,
		TargetType: input.TargetType,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDetachTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	input := struct {
		TargetID   string `json:"target_id"`
		TargetType string `json:"target_type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.service.Taggable.Detach(r.Context(), service.DetachTagArg{
		TagID:      tagId,
		TargetID:   input.TargetID,
		TargetType: input.TargetType,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleGetEdges(w http.ResponseWriter, r *http.Request) {
	edges, err := a.service.Edge.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Edges []database.GetEdgesRow `json:"edges"`
	}

	res := response{
		Edges: edges,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleCreateEdge(w http.ResponseWriter, r *http.Request) {
	input := struct {
		FromID   string `json:"from_id"`
		FromType string `json:"from_type"`
		ToID     string `json:"to_id"`
		ToType   string `json:"to_type"`
		LinkType string `json:"link_type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.service.Edge.Create(r.Context(), service.CreateEdgeArg{
		FromID:   input.FromID,
		FromType: input.FromType,
		ToID:     input.ToID,
		ToType:   input.ToType,
		LinkType: input.LinkType,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDeleteEdge(w http.ResponseWriter, r *http.Request) {
	input := struct {
		FromID   string `json:"from_id"`
		ToID     string `json:"to_id"`
		LinkType string `json:"link_type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.service.Edge.Delete(r.Context(), service.DeleteEdgeArg{
		FromId:   input.FromID,
		ToId:     input.ToID,
		LinkType: input.LinkType,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

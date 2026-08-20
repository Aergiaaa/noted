package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) Pages(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Pages      []pageItem `json:"pages"`
		Page       int        `json:"page"`
		Limit      int        `json:"limit"`
		TotalPages int32      `json:"total_pages"`
	}

	searchQuery := r.URL.Query().Get("q")
	if searchQuery != "" {
		found, err := h.Service.Page.Search(r.Context(), searchQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		res := response{
			Pages:      toPageItems(found),
			Page:       1,
			Limit:      limit,
			TotalPages: 1,
		}

		if err = json.NewEncoder(w).Encode(res); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}

	pages, err := h.Service.Page.GetPagePaginated(r.Context(), page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	totalPages, err := h.Service.Page.GetTotalPage(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res := response{
		Pages:      toPageItems(pages),
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) CreatePage(w http.ResponseWriter, r *http.Request) {
	input := struct {
		Title string `json:"title"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page, err := h.Service.Page.Create(r.Context(), input.Title, nil)
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

func (h *Handler) GetPage(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	page, err := h.Service.Page.GetById(r.Context(), pageId)
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

func (h *Handler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	input := struct {
		Title string `json:"title"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page, err := h.Service.Page.Update(r.Context(), input.Title, pageId, nil)
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

func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	if err := h.Service.Page.Delete(r.Context(), pageId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RestorePage(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	if err := h.Service.Page.Restore(r.Context(), pageId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetPageBlocks(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	blocks, err := h.Service.Block.GetBlocksByPage(r.Context(), pageId)
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

func (h *Handler) GetBacklinks(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	links, err := h.Service.Page.GetBacklinkPages(r.Context(), pageId)
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

func (h *Handler) SyncWikiLinks(w http.ResponseWriter, r *http.Request) {
	pageId := chi.URLParam(r, "id")

	input := struct {
		Titles []string `json:"titles"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Service.Edge.SyncWikiLinks(r.Context(), pageId, input.Titles); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

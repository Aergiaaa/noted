package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/ui/modules"
	"github.com/go-chi/chi"
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

	modules.PageView(page.Title, blocks).Render(r.Context(), w)
}

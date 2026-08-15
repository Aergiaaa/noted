package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	database "github.com/Aergiaaa/noted/internal/database"
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

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Pages []database.GetPagePaginatedRow `json:"pages"`
		Page  int                            `json:"page"`
		Limit int                            `json:"limit"`
	}

	res := response{
		Pages: pages,
		Page:  page,
		Limit: limit,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

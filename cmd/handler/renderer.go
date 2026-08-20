package handler

import (
	"net/http"
	"slices"
	"time"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/service"
	"github.com/Aergiaaa/noted/ui/modules"
	"github.com/Aergiaaa/noted/ui/pages"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) RenderError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	w.WriteHeader(code)
	pages.Error(code, msg).Render(r.Context(), w)
}

func (h *Handler) RenderIndex(w http.ResponseWriter, r *http.Request) {
	pages.Landing().Render(r.Context(), w)
}

func (h *Handler) RenderLogin(w http.ResponseWriter, r *http.Request) {
	pages.Login().Render(r.Context(), w)
}

func (h *Handler) RenderPageFragment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	page, err := h.Service.Page.GetById(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	blocks, err := h.Service.Block.GetBlocksByPage(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balances := []database.GetPocketBalancesRow{}
	for _, b := range blocks {
		if b.Type == "finance" {
			balances, err = h.Service.Pocket.GetBalances(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			break
		}
	}

	txByPocket := map[string][]database.GetTransactionsWithPocketNamesRow{}
	var pocketIds []string
	for _, b := range blocks {
		if b.Type != "finance" {
			continue
		}

		pid := parseFinancePocket(b.Content)
		if pid == "" || slices.Contains(pocketIds, pid) {
			continue
		}

		pocketIds = append(pocketIds, pid)
	}

	for _, pid := range pocketIds {
		txs, err := h.Service.Transaction.GetFiltered(r.Context(), service.FilterTransactionsArg{PocketID: pid})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		txByPocket[pid] = txs
	}

	tags, err := h.Service.Taggable.GetTagsByTargetId(r.Context(), id, "page")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	backlinks, err := h.Service.Page.GetBacklinkPages(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pagesAll, err := h.Service.Page.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageMap := make(map[string]string, len(pagesAll))
	for _, p := range pagesAll {
		pageMap[p.Title] = p.ID.String()
	}

	modules.PageView(page.Title, page.ID.String(), blocks, tags, backlinks, pageMap, balances, txByPocket).Render(r.Context(), w)
}

func (h *Handler) RenderPocketsFragment(w http.ResponseWriter, r *http.Request) {
	pockets, err := h.Service.Pocket.GetBalances(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summary, err := h.Service.Transaction.GetMonthlySummary(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pages.PocketView(pockets, summary).Render(r.Context(), w)
}

func (h *Handler) RenderTransactionsFragment(w http.ResponseWriter, r *http.Request) {
	pocketQuery := r.URL.Query().Get("pocket")
	fromQuery := r.URL.Query().Get("from")
	toQuery := r.URL.Query().Get("to")

	var transactions []database.GetTransactionsWithPocketNamesRow
	var err error

	if pocketQuery != "" || fromQuery != "" || toQuery != "" {
		var fromDate, toDate time.Time
		if fromQuery != "" {
			fromDate, err = time.Parse(time.DateOnly, fromQuery)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if toQuery != "" {
			toDate, err = time.Parse(time.DateOnly, toQuery)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		transactions, err = h.Service.Transaction.GetFiltered(r.Context(), service.FilterTransactionsArg{
			PocketID: pocketQuery,
			FromDate: fromDate,
			ToDate:   toDate,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		transactions, err = h.Service.Transaction.GetWithPockets(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	pockets, err := h.Service.Pocket.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tags := make(map[string][]database.GetTagsByTargetRow)
	for _, t := range transactions {
		rowTags, err := h.Service.Taggable.GetTagsByTargetId(r.Context(), t.ID.String(), "transaction")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tags[t.ID.String()] = rowTags
	}

	pages.TransactionView(transactions, tags, pockets).Render(r.Context(), w)
}

func (h *Handler) RenderTrashFragment(w http.ResponseWriter, r *http.Request) {
	pockets, err := h.Service.Pocket.GetDeleted(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	transactions, err := h.Service.Transaction.GetDeleted(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pages.TrashView(pockets, transactions).Render(r.Context(), w)
}

func (h *Handler) RenderTagPagesFragment(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")
	nameQuery := r.URL.Query().Get("name")

	rows, err := h.Service.Taggable.GetPagesByTag(r.Context(), tagId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pages.TagPagesView(nameQuery, tagId, rows).Render(r.Context(), w)
}

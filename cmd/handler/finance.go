package handler

import (
	"encoding/json"
	"net/http"
	"time"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/service"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) Transactions(w http.ResponseWriter, r *http.Request) {
	transactions, err := h.Service.Transaction.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Transactions []database.GetAllTransactionsRow `json:"transactions"`
	}

	res := response{
		Transactions: transactions,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	transactionId := chi.URLParam(r, "id")

	transaction, err := h.Service.Transaction.GetById(r.Context(), transactionId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID           string         `json:"id"`
		Title        string         `json:"title"`
		Type         string         `json:"type"`
		Amount       pgtype.Numeric `json:"amount"`
		Date         pgtype.Date    `json:"date"`
		FromPocketID pgtype.UUID    `json:"from_pocket_id"`
		ToPocketID   pgtype.UUID    `json:"to_pocket_id"`
	}

	res := response{
		ID:           transaction.ID.String(),
		Title:        transaction.Title,
		Type:         transaction.Type,
		Amount:       transaction.Amount,
		Date:         transaction.Date,
		FromPocketID: transaction.FromPocketID,
		ToPocketID:   transaction.ToPocketID,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	input := struct {
		Title        string  `json:"title"`
		Type         string  `json:"type"`
		Amount       float64 `json:"amount"`
		Date         string  `json:"date"`
		FromPocketID string  `json:"from_pocket_id"`
		ToPocketID   string  `json:"to_pocket_id"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	date, err := time.Parse(time.DateOnly, input.Date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	transaction, err := h.Service.Transaction.Create(r.Context(), service.CreateTransactionArg{
		Title:        input.Title,
		Type:         input.Type,
		Amount:       input.Amount,
		Date:         date,
		FromPocketID: input.FromPocketID,
		ToPocketID:   input.ToPocketID,
	})
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
		ID:    transaction.ID.String(),
		Title: transaction.Title,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	transactionId := chi.URLParam(r, "id")

	input := struct {
		Title        string  `json:"title"`
		Type         string  `json:"type"`
		Amount       float64 `json:"amount"`
		Date         string  `json:"date"`
		FromPocketID string  `json:"from_pocket_id"`
		ToPocketID   string  `json:"to_pocket_id"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	date, err := time.Parse(time.DateOnly, input.Date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	transaction, err := h.Service.Transaction.Update(r.Context(), service.UpdateTransactionArg{
		Title:        input.Title,
		Type:         input.Type,
		Amount:       input.Amount,
		Date:         date,
		FromPocketID: input.FromPocketID,
		ToPocketID:   input.ToPocketID,
		ID:           transactionId,
	})
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
		ID:    transaction.ID.String(),
		Title: transaction.Title,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	transactionId := chi.URLParam(r, "id")

	if err := h.Service.Transaction.Delete(r.Context(), transactionId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RestoreTransaction(w http.ResponseWriter, r *http.Request) {
	transactionId := chi.URLParam(r, "id")

	if err := h.Service.Transaction.Restore(r.Context(), transactionId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Pockets(w http.ResponseWriter, r *http.Request) {
	pockets, err := h.Service.Pocket.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Pockets []database.GetAllPocketsRow `json:"pockets"`
	}

	res := response{
		Pockets: pockets,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) CreatePocket(w http.ResponseWriter, r *http.Request) {
	input := struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pocket, err := h.Service.Pocket.Create(r.Context(), input.Name, input.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}

	res := response{
		ID:   pocket.ID.String(),
		Name: pocket.Name,
		Type: pocket.Type,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) UpdatePocket(w http.ResponseWriter, r *http.Request) {
	pocketId := chi.URLParam(r, "id")

	input := struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pocket, err := h.Service.Pocket.Update(r.Context(), input.Name, input.Type, pocketId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}

	res := response{
		ID:   pocket.ID.String(),
		Name: pocket.Name,
		Type: pocket.Type,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) DeletePocket(w http.ResponseWriter, r *http.Request) {
	pocketId := chi.URLParam(r, "id")

	if err := h.Service.Pocket.Delete(r.Context(), pocketId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RestorePocket(w http.ResponseWriter, r *http.Request) {
	pocketId := chi.URLParam(r, "id")

	if err := h.Service.Pocket.Restore(r.Context(), pocketId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

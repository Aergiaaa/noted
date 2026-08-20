package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Aergiaaa/noted/service"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) CreateBlock(w http.ResponseWriter, r *http.Request) {
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

	block, err := h.Service.Block.Create(r.Context(), service.CreateBlockArgs{
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

func (h *Handler) UpdateBlock(w http.ResponseWriter, r *http.Request) {
	blockId := chi.URLParam(r, "id")

	input := struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	block, err := h.Service.Block.Update(r.Context(), service.UpdateBlockArgs{
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

func (h *Handler) DeleteBlock(w http.ResponseWriter, r *http.Request) {
	blockId := chi.URLParam(r, "id")

	if err := h.Service.Block.Delete(r.Context(), blockId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RestoreBlock(w http.ResponseWriter, r *http.Request) {
	blockId := chi.URLParam(r, "id")

	if err := h.Service.Block.Restore(r.Context(), blockId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderBlocks(w http.ResponseWriter, r *http.Request) {
	input := []service.ReorderBlockArgs{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Service.Block.Reorder(r.Context(), input); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

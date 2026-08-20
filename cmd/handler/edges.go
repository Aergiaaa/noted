package handler

import (
	"encoding/json"
	"net/http"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/service"
)

func (h *Handler) GetEdges(w http.ResponseWriter, r *http.Request) {
	edges, err := h.Service.Edge.GetAll(r.Context())
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

func (h *Handler) CreateEdge(w http.ResponseWriter, r *http.Request) {
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

	if err := h.Service.Edge.Create(r.Context(), service.CreateEdgeArg{
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

func (h *Handler) DeleteEdge(w http.ResponseWriter, r *http.Request) {
	input := struct {
		FromID   string `json:"from_id"`
		ToID     string `json:"to_id"`
		LinkType string `json:"link_type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Service.Edge.Delete(r.Context(), service.DeleteEdgeArg{
		FromId:   input.FromID,
		ToId:     input.ToID,
		LinkType: input.LinkType,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

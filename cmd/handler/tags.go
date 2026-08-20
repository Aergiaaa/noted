package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Aergiaaa/noted/service"
	"github.com/Aergiaaa/noted/ui/pages"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.Service.Tag.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type tagItem struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}

	type response struct {
		Tags []tagItem `json:"tags"`
	}

	items := make([]tagItem, len(tags))
	for i, t := range tags {
		items[i] = tagItem{ID: t.ID.String(), Name: t.Name, Color: t.Color}
	}

	res := response{
		Tags: items,
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	input := struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tag, err := h.Service.Tag.Create(r.Context(), input.Name, input.Color)
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

func (h *Handler) UpdateTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	input := struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tag, err := h.Service.Tag.Update(r.Context(), input.Name, input.Color, tagId)
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

func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	if err := h.Service.Tag.Delete(r.Context(), tagId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RestoreTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	if err := h.Service.Tag.Restore(r.Context(), tagId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AttachTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	input := struct {
		TargetID   string `json:"target_id"`
		TargetType string `json:"target_type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Service.Taggable.Attach(r.Context(), service.AttachTagArg{
		TagID:      tagId,
		TargetID:   input.TargetID,
		TargetType: input.TargetType,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DetachTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	input := struct {
		TargetID   string `json:"target_id"`
		TargetType string `json:"target_type"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Service.Taggable.Detach(r.Context(), service.DetachTagArg{
		TagID:      tagId,
		TargetID:   input.TargetID,
		TargetType: input.TargetType,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetPagesByTag(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")

	pages, err := h.Service.Taggable.GetPagesByTag(r.Context(), tagId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Pages []pageItem `json:"pages"`
	}

	res := response{
		Pages: toPageItems(pages),
	}

	if err = json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) TagPagesFragment(w http.ResponseWriter, r *http.Request) {
	tagId := chi.URLParam(r, "id")
	nameQuery := r.URL.Query().Get("name")

	rows, err := h.Service.Taggable.GetPagesByTag(r.Context(), tagId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pages.TagPagesView(nameQuery, tagId, rows).Render(r.Context(), w)
}

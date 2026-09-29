package handler

import (
	"encoding/json"
	"net/http"
)

// Healthz handles GET /healthz: liveness probe, no auth (F1).
func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

package handler

import (
	"errors"
	"log"
	"net/http"

	"noted/internal/auth"
)

// loginRequest is the request body of POST /api/auth/login (API.md):
//
//	{"code": "123456"}          — live 6-digit TOTP
//	{"code": "ABCD-EFGH"}       — one unused recovery code
//
// It is the whole contract the frontend sends; DecodeJSON rejects any
// unknown key, so a drifted shape fails with 400 instead of silently
// ignoring fields.
type loginRequest struct {
	Code string `json:"code"`
}

// Login: 401 for every credential failure — generic by design
// (SECURITY.md). 429 comes from middleware.RateLimit in front of this.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if !DecodeJSON(w, r, &in) {
		return
	}
	if in.Code == "" {
		WriteError(w, http.StatusBadRequest, "code is required", map[string]string{"code": "required"})
		return
	}
	token, err := h.auth.Login(r.Context(), in.Code)
	switch {
	case errors.Is(err, auth.ErrInvalidCode):
		WriteError(w, http.StatusUnauthorized, "invalid code", nil)
	case err != nil:
		log.Printf("handler: login: %v", err)
		WriteError(w, http.StatusInternalServerError, "", nil)
	default:
		h.setSessionCookie(w, token)
		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(auth.SESSION_COOKIE)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "", nil)
		return
	}
	expires, err := h.auth.Validate(r.Context(), c.Value)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "", nil)
		return
	}
	h.setSessionCookie(w, c.Value)
	WriteJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"expires_at":    expires.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Logout is public and idempotent: a failed delete must 500 with the
// cookie kept, otherwise the session would survive server-side.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var token string
	if c, err := r.Cookie(auth.SESSION_COOKIE); err == nil {
		token = c.Value
	}
	if err := h.auth.Logout(r.Context(), token); err != nil {
		log.Printf("handler: logout: %v", err)
		WriteError(w, http.StatusInternalServerError, "", nil)
		return
	}
	h.setSessionCookie(w, "")
	w.WriteHeader(http.StatusNoContent)
}

// RecoveryCodes: RequireSession already ran, so any failure here is
// internal — 500, codes shown once, never logged.
func (h *Handler) RecoveryCodes(w http.ResponseWriter, r *http.Request) {
	codes, err := h.auth.RegenerateRecovery(r.Context())
	if err != nil {
		log.Printf("handler: recovery codes: %v", err)
		WriteError(w, http.StatusInternalServerError, "", nil)
		return
	}
	WriteJSON(w, http.StatusOK, map[string][]string{"recovery_codes": codes})
}

// An empty token clears the cookie: Max-Age < 0 tells the browser to drop it.
func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	maxAge := auth.COOKIE_MAX_AGE_SEC
	if token == "" {
		maxAge = -1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SESSION_COOKIE,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

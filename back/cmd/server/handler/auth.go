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
	var loginReq loginRequest
	if !DecodeJSON(w, r, &loginReq) {
		return
	}
	if loginReq.Code == "" {
		WriteError(w, http.StatusBadRequest, "code is required", map[string]string{"code": "required"})
		return
	}
	token, err := h.auth.Login(r.Context(), loginReq.Code)
	switch {
	case errors.Is(err, auth.ErrInvalidCode):
		WriteError(w, http.StatusUnauthorized, "invalid code", nil)
	case err != nil:
		log.Printf("handler: login: %v", err)
		WriteError(w, http.StatusInternalServerError, "", nil)
	default:
		auth.WriteSessionCookie(w, token, h.secure)
		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// Session validates the cookie and re-issues it so the browser-side
// Max-Age keeps pace with the server-side slide.
func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	token, expires, err := h.auth.ValidateRequest(r)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "", nil)
		return
	}
	auth.WriteSessionCookie(w, token, h.secure)
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
	auth.WriteSessionCookie(w, "", h.secure)
	w.WriteHeader(http.StatusNoContent)
}

// RecoveryCodes: RequireSession already proved the session exists, so the
// failures left here are the step-up refusal (403: the session itself was
// minted from a recovery code), a session that died between guard and
// handler (401), and internal ones (500). Codes are shown once, never
// logged.
func (h *Handler) RecoveryCodes(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(auth.SESSION_COOKIE)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "", nil)
		return
	}
	codes, err := h.auth.RotateRecovery(r.Context(), c.Value)
	switch {
	case errors.Is(err, auth.ErrStepUpRequired):
		WriteErrorCode(w, http.StatusForbidden, "step_up_required", "TOTP login required", nil)
	case errors.Is(err, auth.ErrUnauthenticated):
		WriteError(w, http.StatusUnauthorized, "", nil)
	case err != nil:
		log.Printf("handler: recovery codes: %v", err)
		WriteError(w, http.StatusInternalServerError, "", nil)
	default:
		WriteJSON(w, http.StatusOK, map[string][]string{"recovery_codes": codes})
	}
}

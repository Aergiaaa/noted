package handler

import (
	"errors"
	"log"
	"net/http"

	"noted/internal/auth"
)

// Login — POST /api/auth/login {code}: a 6-digit TOTP or a recovery code
// mints the session cookie. 400 when code is missing (API.md required
// field), 401 "invalid code" for every credential failure (generic by
// design — SECURITY.md), 500 only when the session could not be written.
// 429 + Retry-After comes from middleware.RateLimit in front of this.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
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

// Session — GET /api/session: the router guard's probe. Valid cookie →
// 200 {authenticated, expires_at} with the sliding expiry re-issued in the
// cookie too; anything else → the plain 401 envelope (API.md).
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

// Logout — POST /api/auth/logout: public and idempotent (no session
// required — an absent or stale cookie still clears). The row delete must
// succeed, otherwise the session would survive server-side: 500, cookie
// kept. 204 + cleared cookie otherwise (API.md).
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

// RecoveryCodes — POST /api/auth/recovery-codes (session required): burns
// the whole unused pool and returns 10 fresh codes once. The route's
// RequireSession guard runs first, so a non-401 here is an internal
// failure (500); the codes are shown exactly once, never logged.
func (h *Handler) RecoveryCodes(w http.ResponseWriter, r *http.Request) {
	codes, err := h.auth.RegenerateRecovery(r.Context())
	if err != nil {
		log.Printf("handler: recovery codes: %v", err)
		WriteError(w, http.StatusInternalServerError, "", nil)
		return
	}
	WriteJSON(w, http.StatusOK, map[string][]string{"recovery_codes": codes})
}

// setSessionCookie issues the noted_session cookie: a token sets the
// 30d Max-Age attributes from SECURITY.md; an empty token clears it
// (Max-Age < 0 tells the browser to drop the cookie).
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

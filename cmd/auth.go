package main

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/pquerna/otp/totp"
)

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, err := a.store.Get(r, "session")
	if err != nil {
		a.handle.RenderError(w, r, http.StatusBadRequest, "Error: cannot fetch session")
		return
	}

	session.Options.MaxAge = -1

	session.Save(r, w)

	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

func (a *App) handleVerify(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")

	isValid := totp.Validate(code, a.secret)
	if !isValid {
		a.handle.RenderError(w, r, http.StatusUnauthorized, "Error: wrong otp")
		return
	}

	session, err := a.store.Get(r, "session")
	if err != nil {
		a.handle.RenderError(w, r, http.StatusBadRequest, "Error: cannot fetch session")
		return
	}

	session.Values["authenticated"] = true

	session.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 7, // 7 days
	}

	if err := session.Save(r, w); err != nil {
		a.handle.RenderError(w, r, http.StatusInternalServerError, "Error: cannot create session")
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := a.store.Get(r, "session")
		if err != nil {
			a.handle.RenderError(w, r, http.StatusBadRequest, "Error: cannot fetch session")
			return
		}

		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

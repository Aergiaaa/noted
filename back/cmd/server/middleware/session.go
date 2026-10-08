package middleware

import (
	"net/http"

	"noted/cmd/server/handler"
	"noted/internal/auth"
)

func RequireSession(svc *auth.Service) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requireSession(svc, w, r, next)
		})
	}
}

func requireSession(svc *auth.Service, w http.ResponseWriter, r *http.Request, next http.Handler) {
	if _, err := svc.ValidateRequest(r); err != nil {
		handler.WriteError(w, http.StatusUnauthorized, "", nil)
		return
	}
	next.ServeHTTP(w, r)
}

package middleware

import (
	"net/http"

	"noted/cmd/server/handler"
	"noted/internal/auth"
)

func RequireSession(svc *auth.Service, secure bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requireSession(svc, secure, w, r, next)
		})
	}
}

func requireSession(svc *auth.Service, secure bool, w http.ResponseWriter, r *http.Request, next http.Handler) {
	token, _, err := svc.ValidateRequest(r)
	if err != nil {
		handler.WriteError(w, http.StatusUnauthorized, "", nil)
		return
	}

	// Re-issue on every guarded response: the browser cookie's Max-Age
	// only moves where a Set-Cookie goes out (SECURITY.md, cookie
	// freshness), and the value is the same token — just re-lifespanned.
	auth.WriteSessionCookie(w, token, secure)

	next.ServeHTTP(w, r)
}

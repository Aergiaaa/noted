package middleware

import (
	"net/http"

	"noted/cmd/server/handler"
	"noted/internal/auth"
)

// RequireSession gates every authenticated route: a valid noted_session
// cookie continues to the handler; anything else — no cookie, unknown,
// expired, or an internal lookup failure — gets the frozen 401 envelope
// before a handler can run (F4 / SECURITY.md). Details stay in the server
// log via the auth service.
func RequireSession(svc *auth.Service) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requireSession(svc, w, r, next)
		})
	}
}

// requireSession is RequireSession's handler body, split so tests can drive
// it directly without a chi context.
func requireSession(svc *auth.Service, w http.ResponseWriter, r *http.Request, next http.Handler) {
	if _, err := svc.ValidateRequest(r); err != nil {
		handler.WriteError(w, http.StatusUnauthorized, "", nil)
		return
	}
	next.ServeHTTP(w, r)
}

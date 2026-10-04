package main

import (
	"net/http"
	"time"

	"noted/cmd/server/handler"
	"noted/cmd/server/middleware"
	"noted/internal/auth"

	"github.com/go-chi/chi/v5"
)

// newRouter builds the HTTP router: the F3 middleware chain first (so
// 404/405 responses also get headers, a request id, and a log line),
// then the route table. Handler methods live in handler/, grouped per
// endpoint family; chain order matters — RequestID before AccessLog so the
// log can quote the id, Recover after AccessLog so the request line records
// the panic's 500, Forwarded before SecurityHeaders because HSTS trusts
// its verdict, Origin early so foreign mutations never reach a handler.
func (a *app) newRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(
		middleware.RequestID,
		middleware.AccessLog,
		middleware.Recover,
		middleware.Forwarded(a.conf.TrustedProxies),
		middleware.SecurityHeaders,
		middleware.BodyLimit,
		middleware.OriginCheck(a.conf.AppOrigin),
	)

	// 405 stays on chi's default: it is outside API.md's frozen envelope
	// statuses, and only chi's default handler emits the RFC-required
	// Allow header (it knows the methods that would have matched).
	r.NotFound(handleWriteErrorNotFound)

	r.Get("/healthz", a.handler.Healthz)

	r.Route("/api", func(r chi.Router) {
		// Login is rate-limited per client IP (every attempt counts, 6th
		// → 429 + Retry-After); logout and the session probe are public —
		// they answer 401/204 correctly with or without a cookie.
		limiter := middleware.NewTokenBucket(auth.LOGIN_RATE_LIMIT, auth.LOGIN_RATE_WINDOW, time.Now)
		r.With(middleware.RateLimit(limiter)).Post("/auth/login", a.handler.Login)
		r.Post("/auth/logout", a.handler.Logout)
		r.Get("/session", a.handler.Session)

		// Everything else requires the session cookie. Domain routes
		// (events/notes/tasks/finance/dashboard, F6+) join this group.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireSession(a.auth))
			r.Post("/auth/recovery-codes", a.handler.RecoveryCodes)
		})
	})

	return r
}

func handleWriteErrorNotFound(w http.ResponseWriter, _ *http.Request) {
	handler.WriteError(w, http.StatusNotFound, "", nil)
}

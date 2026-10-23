package main

import (
	"net/http"
	"time"

	"noted/cmd/server/handler"
	"noted/cmd/server/middleware"
	"noted/internal/auth"

	"github.com/go-chi/chi/v5"
)

// newRouter: the F3 chain runs first so 404/405 responses also get headers,
// an id and a log line. Order matters — RequestID before AccessLog, Recover
// after it (so the request line records the panic's 500), Forwarded before
// SecurityHeaders (HSTS trusts its verdict), Origin early so foreign
// mutations never reach a handler.
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

	// 405 stays on chi's default: it is outside API.md's frozen envelope,
	// and only chi's handler emits the RFC-required Allow header.
	r.NotFound(handleWriteErrorNotFound)

	r.Get("/healthz", a.handler.Healthz)

	r.Route("/api", func(r chi.Router) {
		limiter := middleware.NewTokenBucket(
			auth.LOGIN_RATE_LIMIT,
			auth.LOGIN_RATE_WINDOW,
			time.Now,
		)
		r.With(middleware.RateLimit(limiter)).
			Post("/auth/login", a.handler.Login)

		// Logout and the session probe stay public: both answer correctly
		// with or without a cookie.
		r.Post("/auth/logout", a.handler.Logout)
		r.Get("/session", a.handler.Session)

		// Domain routes (dashboard joins in F10) live in
		// this group, behind the session guard.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireSession(a.auth, a.secure))

			r.Post("/auth/recovery-codes", a.handler.RecoveryCodes)

			r.Get("/events", a.handler.ListEvents)
			r.Post("/events", a.handler.CreateEvent)
			r.Patch("/events/{id}", a.handler.UpdateEvent)
			r.Delete("/events/{id}", a.handler.DeleteEvent)
			r.Post("/events/{id}/restore", a.handler.RestoreEvent)

			r.Get("/notes", a.handler.ListNotes)
			r.Post("/notes", a.handler.CreateNote)
			r.Patch("/notes/{id}", a.handler.UpdateNote)
			r.Delete("/notes/{id}", a.handler.DeleteNote)

			r.Get("/tasks", a.handler.ListTasks)
			r.Post("/tasks", a.handler.CreateTask)
			r.Patch("/tasks/{id}", a.handler.UpdateTask)
			r.Delete("/tasks/{id}", a.handler.DeleteTask)

			r.Get("/finance/balance", a.handler.FinanceBalance)
			r.Get("/finance/wallets", a.handler.ListWallets)
			r.Post("/finance/wallets", a.handler.CreateWallet)
			r.Patch("/finance/wallets/{id}", a.handler.UpdateWallet)
			r.Get("/finance/transactions", a.handler.ListTransactions)
			r.Post("/finance/transactions", a.handler.CreateTransaction)
			r.Post("/finance/transfers", a.handler.CreateTransfer)
			r.Patch("/finance/transactions/{id}", a.handler.UpdateTransaction)
			r.Delete("/finance/transactions/{id}", a.handler.DeleteTransaction)
		})
	})

	return r
}

func handleWriteErrorNotFound(w http.ResponseWriter, _ *http.Request) {
	handler.WriteError(w, http.StatusNotFound, "", nil)
}

package handler

import (
	"noted/internal/auth"
	"noted/internal/service"
)

type Handler struct {
	auth   *auth.Service
	events *service.EventService
	notes  *service.NoteService
	tasks  *service.TaskService
	// finance spans two concrete services (wallets + transactions); both
	// share one *Queries and stay separate — no umbrella service (D2).
	wallets      *service.WalletService
	transactions *service.TransactionService

	// secure pins the cookie Secure attribute to APP_ENV=prod — plain-HTTP
	// localhost must still receive the cookie in dev (SECURITY.md).
	secure bool
}

// New: auth and any single domain service may be nil in tests that never
// touch those endpoints — every nil is exercised only by its own tests.
func New(a *auth.Service, events *service.EventService, notes *service.NoteService, tasks *service.TaskService, wallets *service.WalletService, transactions *service.TransactionService, secure bool) *Handler {
	return &Handler{
		auth:         a,
		events:       events,
		notes:        notes,
		tasks:        tasks,
		wallets:      wallets,
		transactions: transactions,
		secure:       secure,
	}
}

// derefParam collapses a decoded optional request field to the plain
// string the service inputs take (nil → ""). PATCH requests keep the
// pointer — absent must stay distinguishable from "" there — but create
// inputs resolve once here, at the boundary.
func derefParam(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Package handler holds the HTTP handler layer: parse/validate, call one
// service method, map to status/JSON (see ARCHITECTURE.md). One file per
// endpoint (healthz.go, ...); cohesive endpoints may share a file
// (auth.go = login/logout/session/recovery-codes). Route registration
// stays in the server's router.go.
package handler

import "noted/internal/auth"

// Handler carries the handler layer's dependencies — the auth service
// (F4) with more services to come. The constructor is the wiring seam so
// router.go and main.go change only when a dependency lands.
type Handler struct {
	auth *auth.Service

	// secure pins the cookie Secure attribute to APP_ENV=prod (SECURITY.md:
	// plain-HTTP localhost must still receive the cookie in dev).
	secure bool
}

// New builds a Handler over its dependencies. auth may be nil in tests
// that never touch authenticated endpoints.
func New(a *auth.Service, secure bool) *Handler {
	return &Handler{auth: a, secure: secure}
}

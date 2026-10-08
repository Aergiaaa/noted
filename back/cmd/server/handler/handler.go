package handler

import "noted/internal/auth"

type Handler struct {
	auth *auth.Service

	// secure pins the cookie Secure attribute to APP_ENV=prod — plain-HTTP
	// localhost must still receive the cookie in dev (SECURITY.md).
	secure bool
}

// New: auth may be nil in tests that never touch authenticated endpoints.
func New(a *auth.Service, secure bool) *Handler {
	return &Handler{auth: a, secure: secure}
}

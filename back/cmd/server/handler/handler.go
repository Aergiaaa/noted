package handler

import (
	"noted/internal/auth"
	"noted/internal/service"
)

type Handler struct {
	auth   *auth.Service
	events *service.EventService

	// secure pins the cookie Secure attribute to APP_ENV=prod — plain-HTTP
	// localhost must still receive the cookie in dev (SECURITY.md).
	secure bool
}

// New: auth may be nil in tests that never touch authenticated endpoints.
func New(a *auth.Service, events *service.EventService, secure bool) *Handler {
	return &Handler{auth: a, events: events, secure: secure}
}

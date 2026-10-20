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

	// secure pins the cookie Secure attribute to APP_ENV=prod — plain-HTTP
	// localhost must still receive the cookie in dev (SECURITY.md).
	secure bool
}

// New: auth and any single domain service may be nil in tests that never
// touch those endpoints — every nil is exercised only by its own tests.
func New(a *auth.Service, events *service.EventService, notes *service.NoteService, tasks *service.TaskService, secure bool) *Handler {
	return &Handler{auth: a, events: events, notes: notes, tasks: tasks, secure: secure}
}

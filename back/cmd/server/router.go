package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// newRouter builds the HTTP router. Route registration lives here so the
// table grows in one file; handler methods live in handler/ (one file per
// endpoint). F1: only GET /healthz (no auth).
func (a *app) newRouter() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", a.handler.Healthz)

	return r
}

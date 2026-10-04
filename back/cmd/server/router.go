package main

import (
	"net/http"

	"noted/cmd/server/handler"
	"noted/cmd/server/middleware"

	"github.com/go-chi/chi/v5"
)

// newRouter builds the HTTP router: the F3 middleware chain first (so
// 404/405 responses also get headers, a request id, and a log line),
// then the route table. Handler methods live in handler/, one file per
// endpoint; chain order matters — RequestID before AccessLog so the log
// can quote the id, Recover after AccessLog so the request line records
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

	return r
}

func handleWriteErrorNotFound(w http.ResponseWriter, _ *http.Request) {
	handler.WriteError(w, http.StatusNotFound, "", nil)
}

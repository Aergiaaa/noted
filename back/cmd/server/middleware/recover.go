package middleware

import (
	"errors"
	"log"
	"net/http"
	"runtime/debug"

	"noted/cmd/server/handler"
)

// Recover turns a handler panic into the frozen 500 envelope instead of
// a dropped connection: without it net/http aborts the request with no
// response body and no access-log line. The panic value and stack go to
// the server log with the request id (SECURITY.md: details with
// request_id in server logs; internals never reach the client).
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tw := &trackingWriter{
			ResponseWriter: w,
		}

		defer recoverHandler(r, tw)

		next.ServeHTTP(tw, r)
	})
}

// recoverHandler is Recover's handler body: run the handler under a
// recover, then either re-panic control-flow signals untouched or turn
// the panic into a logged 500.
func recoverHandler(r *http.Request, tw *trackingWriter) {
	rec := recover()
	if rec == nil {
		return
	}

	if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		panic(rec) // deliberate abort: net/http swallows it without a stack trace
	}

	log.Printf("panic rid=%s method=%s uri=%s: %v\n%s",
		RequestIDFrom(r), r.Method, r.URL.RequestURI(), rec, debug.Stack())

	if tw.written {
		// Status already went out; the body is truncated, so abort the
		// response rather than let the client mistake it for complete.
		panic(http.ErrAbortHandler)
	}

	handler.WriteError(tw, http.StatusInternalServerError, "", nil)
}

// trackingWriter records whether a status hit the wire, so Recover knows
// when the 500 envelope can no longer be written.
type trackingWriter struct {
	http.ResponseWriter
	written bool
}

// implementation for response writer
func (tw *trackingWriter) WriteHeader(code int) {
	tw.written = true
	tw.ResponseWriter.WriteHeader(code)
}

// implementation for response writer
func (tw *trackingWriter) Write(b []byte) (int, error) {
	tw.written = true
	return tw.ResponseWriter.Write(b)
}

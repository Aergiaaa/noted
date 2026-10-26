package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"noted/cmd/server/handler"
)

// Recover turns a handler panic into the frozen 500 envelope: without it,
// net/http aborts the request with no body and no access-log line. The
// panic value and stack go to the server log with the request id — never
// to the client (SECURITY.md).
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tw := &trackingWriter{
			ResponseWriter: w,
		}

		defer recoverHandler(r, tw)

		next.ServeHTTP(tw, r)
	})
}

func recoverHandler(r *http.Request, tw *trackingWriter) {
	rec := recover()
	if rec == nil {
		return
	}

	if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		panic(rec) // deliberate abort: net/http swallows it without a stack trace
	}

	slog.Error("panic",
		"rid", RequestIDFrom(r),
		"method", r.Method,
		"uri", r.URL.RequestURI(),
		"value", rec,
		"stack", string(debug.Stack()),
	)

	if tw.written {
		// Status already went out; the body is truncated, so abort rather
		// than let the client mistake it for complete.
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

func (tw *trackingWriter) WriteHeader(code int) {
	tw.written = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *trackingWriter) Write(b []byte) (int, error) {
	tw.written = true
	return tw.ResponseWriter.Write(b)
}

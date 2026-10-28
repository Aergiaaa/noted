package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// No bodies, cookies, or query values beyond the URI — SECURITY.md bans
// logging secrets.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			accessLog(w, r, next)
		},
	)
}

// The emit is deferred so a re-panic (Recover's truncated-response path)
// still logs the line for the request.
func accessLog(w http.ResponseWriter, r *http.Request, next http.Handler) {
	start := time.Now()

	sw := &statusWriter{
		ResponseWriter: w,
	}

	defer logStatus(r, sw, start)

	next.ServeHTTP(sw, r)
}

func logStatus(r *http.Request, sw *statusWriter, start time.Time) {
	stat := sw.status
	if stat == 0 {
		stat = http.StatusOK // handler returned without writing
	}

	slog.Info("request",
		"rid", RequestIDFrom(r),
		"method", r.Method,
		"uri", r.URL.RequestURI(),
		"status", stat,
		"dur", time.Since(start),
	)
}

// statusWriter records the first status written, so the log reports what
// the client actually received; later WriteHeader calls belong to the
// underlying writer's superfluous-call problem, not this one's.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	if sw.status == 0 {
		sw.status = code
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if sw.status == 0 {
		sw.status = http.StatusOK
	}
	return sw.ResponseWriter.Write(b)
}

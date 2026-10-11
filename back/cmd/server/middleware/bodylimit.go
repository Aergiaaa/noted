package middleware

import "net/http"

// MAX_BODY_BYTES caps every request body (SECURITY.md: 413 over 1MB).
// Wrapping here means no endpoint can forget the limit; the envelope is
// produced when a handler reads past the cap (handler.DecodeJSON).
const MAX_BODY_BYTES = 1 << 20

func BodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyLimit(w, r, next)
	})
}

func bodyLimit(w http.ResponseWriter, r *http.Request, next http.Handler) {
	r.Body = http.MaxBytesReader(w, r.Body, MAX_BODY_BYTES)
	next.ServeHTTP(w, r)
}

package middleware

import "net/http"

// MaxBodyBytes caps every request body (SECURITY.md: 413 over 1MB).
// Wrapping here means no endpoint can forget the limit; the 413 envelope
// is produced when a handler reads past the cap via handler.DecodeJSON.
const MaxBodyBytes = 1 << 20

// BodyLimit installs http.MaxBytesReader for the request. The cap trips
// on read, so a GET/HEAD with no body is unaffected.
func BodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

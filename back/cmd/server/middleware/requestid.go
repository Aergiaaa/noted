package middleware

import (
	"context"
	"crypto/rand"
	"net/http"
)

// RequestIDHeader echoes an id on every response so a client (or log
// reader) can correlate a failure with the matching server log line
// (SECURITY.md "X-Request-ID echoed").
const RequestIDHeader = "X-Request-ID"

// maxRequestIDLen bounds client-supplied ids so the header can't be
// abused as an unbounded log field.
const maxRequestIDLen = 64

// RequestID accepts a syntactically safe inbound X-Request-ID or mints a
// fresh one, stashes it in the request context for AccessLog, and echoes
// it on the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := validRequestID(r.Header.Get(RequestIDHeader))
		if id == "" {
			id = rand.Text()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// validRequestID returns s when it is a plain token (alphanumeric, "-",
// "_") within the length cap, otherwise "". Rejecting arbitrary input
// keeps client-controlled bytes out of the response header and logs.
func validRequestID(s string) string {
	if s == "" || len(s) > maxRequestIDLen {
		return ""
	}
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return ""
		}
	}
	return s
}

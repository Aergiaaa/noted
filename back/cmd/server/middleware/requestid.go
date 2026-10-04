package middleware

import (
	"context"
	"crypto/rand"
	"net/http"
)

// REQUEST_ID_HEADER echoes an id on every response so a client (or log
// reader) can correlate a failure with the matching server log line
// (SECURITY.md "X-Request-ID echoed").
const REQUEST_ID_HEADER = "X-Request-ID"

// MAX_REQUEST_ID_LEN bounds client-supplied ids so the header can't be
// abused as an unbounded log field.
const MAX_REQUEST_ID_LEN = 64

// RequestID accepts a syntactically safe inbound X-Request-ID or mints a
// fresh one, stashes it in the request context for AccessLog, and echoes
// it on the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID(w, r, next)
	})
}

// requestID is RequestID's handler body: validate or mint the id, echo it
// on the response, and pass the enriched request down the chain.
func requestID(w http.ResponseWriter, r *http.Request, next http.Handler) {
	id := validRequestID(r.Header.Get(REQUEST_ID_HEADER))
	if id == "" {
		id = rand.Text()
	}
	w.Header().Set(REQUEST_ID_HEADER, id)
	next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), REQUEST_ID_KEY, id)))
}

// validRequestID returns s when it is a plain token (alphanumeric, "-",
// "_") within the length cap, otherwise "". Rejecting arbitrary input
// keeps client-controlled bytes out of the response header and logs.
func validRequestID(s string) string {
	if s == "" || len(s) > MAX_REQUEST_ID_LEN {
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

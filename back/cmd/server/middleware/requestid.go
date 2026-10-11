package middleware

import (
	"context"
	"crypto/rand"
	"net/http"
)

// REQUEST_ID_HEADER is echoed on every response so a client (or log
// reader) can correlate a failure with the matching server log line.
const REQUEST_ID_HEADER = "X-Request-ID"

// MAX_REQUEST_ID_LEN bounds client-supplied ids so the header cannot be
// abused as an unbounded log field.
const MAX_REQUEST_ID_LEN = 64

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID(w, r, next)
	})
}

func requestID(w http.ResponseWriter, r *http.Request, next http.Handler) {
	id := validRequestID(r.Header.Get(REQUEST_ID_HEADER))
	if id == "" {
		id = rand.Text()
	}
	w.Header().Set(REQUEST_ID_HEADER, id)
	next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), REQUEST_ID_KEY, id)))
}

// validRequestID rejects anything but a plain token within the cap, which
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

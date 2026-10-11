package middleware

import (
	"net/http"
)

// Middleware is Chi's decorator signature: configurable middlewares
// (Forwarded, OriginCheck) return it from their factory so r.Use accepts
// both them and plain middlewares without conversion.
type Middleware func(http.Handler) http.Handler

// ctxKey is the private context key type; its zero value collides with no
// other package's keys.
type ctxKey int

const (
	REQUEST_ID_KEY ctxKey = iota
	CLIENT_IP_KEY
	SECURE_KEY
)

func RequestIDFrom(r *http.Request) string {
	id, _ := r.Context().Value(REQUEST_ID_KEY).(string)

	return id
}

func ClientIP(r *http.Request) string {
	ip, _ := r.Context().Value(CLIENT_IP_KEY).(string)

	return ip
}

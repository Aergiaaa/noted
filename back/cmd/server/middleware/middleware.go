// Package middleware holds the cross-cutting HTTP chain: request ids,
// access logging, panic recovery, security headers, body limits, origin
// checks, and trusted-proxy resolution (ARCHITECTURE.md layer table). It
// has no domain logic; envelope errors are delegated to the handler
// package.
package middleware

import (
	"net/http"
)

// Middleware is Chi's decorator signature: take a handler, return the
// wrapped one. Configurable middlewares (Forwarded, OriginCheck) return
// it from their factory; plain middlewares already have exactly this
// signature, so r.Use accepts both without conversion.
type Middleware func(http.Handler) http.Handler

// ctxKey is the private context key type; zero value collides with no
// other package's keys.
type ctxKey int

const (
	REQUEST_ID_KEY ctxKey = iota
	CLIENT_IP_KEY
	SECURE_KEY
)

// RequestIDFrom returns the id the RequestID middleware stored, or ""
// when the chain did not run.
func RequestIDFrom(r *http.Request) string {
	id, _ := r.Context().Value(REQUEST_ID_KEY).(string)

	return id
}

// ClientIP returns the peer address as resolved by Forwarded: the direct
// remote address, or the forwarded client when the remote is a trusted
// proxy. "" when Forwarded did not run. Feeds F4's rate limiting.
func ClientIP(r *http.Request) string {
	ip, _ := r.Context().Value(CLIENT_IP_KEY).(string)

	return ip
}

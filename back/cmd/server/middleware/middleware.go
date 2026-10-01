// Package middleware holds the cross-cutting HTTP chain: request ids,
// access logging, security headers, body limits, origin checks, and
// trusted-proxy resolution (ARCHITECTURE.md layer table). It has no
// domain logic; envelope errors are delegated to the handler package.
package middleware

import (
	"net/http"
)

// ctxKey is the private context key type; zero value collides with no
// other package's keys.
type ctxKey int

const (
	requestIDKey ctxKey = iota
	clientIPKey
	secureKey
)

// RequestIDFrom returns the id the RequestID middleware stored, or ""
// when the chain did not run.
func RequestIDFrom(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey).(string)
	return id
}

// ClientIP returns the peer address as resolved by Forwarded: the direct
// remote address, or the forwarded client when the remote is a trusted
// proxy. "" when Forwarded did not run. Feeds F4's rate limiting.
func ClientIP(r *http.Request) string {
	ip, _ := r.Context().Value(clientIPKey).(string)
	return ip
}

// IsSecure reports whether a trusted proxy forwarded the request over
// HTTPS. Direct connections are never secure here: the app itself speaks
// plain HTTP (SECURITY.md).
func IsSecure(r *http.Request) bool {
	secure, _ := r.Context().Value(secureKey).(bool)
	return secure
}

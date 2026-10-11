package middleware

import (
	"fmt"
	"net/http"
)

// HSTS max-age in seconds (2 years).
const HSTS_MAX_AGE = 63072000

// SecurityHeaders sets SECURITY.md's table on every response, including
// 404/405 — the chain runs before route matching. HSTS is the exception:
// only when a trusted proxy said the request arrived over HTTPS, never on
// plain-HTTP dev connections.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			securityHeaders(w, r, next)
		},
	)
}

func securityHeaders(w http.ResponseWriter, r *http.Request, next http.Handler) {
	h := w.Header()

	h.Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("X-Frame-Options", "DENY")

	if IsSecure(r) {
		h.Set("Strict-Transport-Security",
			fmt.Sprintf("max-age=%d; includeSubDomains", HSTS_MAX_AGE))
	}

	next.ServeHTTP(w, r)
}

// IsSecure: direct connections are never secure here — the app itself
// speaks plain HTTP and only a trusted proxy terminates TLS (SECURITY.md).
func IsSecure(r *http.Request) bool {
	secure, _ := r.Context().Value(SECURE_KEY).(bool)

	return secure
}

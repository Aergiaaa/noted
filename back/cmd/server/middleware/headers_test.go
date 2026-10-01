package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// wantHeaders mirrors SECURITY.md's "HTTP headers" table line for line;
// a doc change must fail here until both agree.
var wantHeaders = map[string]string{
	"Content-Security-Policy":      "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
	"X-Content-Type-Options":       "nosniff",
	"Referrer-Policy":              "no-referrer",
	"Permissions-Policy":           "camera=(), microphone=(), geolocation=()",
	"Cross-Origin-Opener-Policy":   "same-origin",
	"Cross-Origin-Resource-Policy": "same-origin",
	"X-Frame-Options":              "DENY",
}

const hstsValue = "max-age=63072000; includeSubDomains"

func serveSecurity(next http.Handler, modify func(*http.Request)) *httptest.ResponseRecorder {
	if next == nil {
		next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	if modify != nil {
		modify(req)
	}
	rec := httptest.NewRecorder()
	SecurityHeaders(next).ServeHTTP(rec, req)
	return rec
}

func TestSecurityHeaders_staticTableSet(t *testing.T) {
	rec := serveSecurity(nil, nil)

	for name, want := range wantHeaders {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestSecurityHeaders_noHSTSWithoutTrustedProxy(t *testing.T) {
	// Direct connection claiming https: the header is untrusted, so HSTS
	// must stay off (dev is plain HTTP; only Caddy may say otherwise).
	rec := serveSecurity(nil, func(r *http.Request) {
		r.Header.Set("X-Forwarded-Proto", "https")
	})

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS = %q, want unset for untrusted peer", got)
	}
}

func TestSecurityHeaders_HSTSBehindTrustedHTTPProxy(t *testing.T) {
	rec := serveSecurityTrusted("https")

	if got := rec.Header().Get("Strict-Transport-Security"); got != hstsValue {
		t.Fatalf("HSTS = %q, want %q", got, hstsValue)
	}
}

func TestSecurityHeaders_trustedPeerWithoutHTTPSEmitsNoHSTS(t *testing.T) {
	rec := serveSecurityTrusted("http")

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS = %q, want unset for plain-http forward", got)
	}
}

// serveSecurityTrusted runs Forwarded(192.0.2.1 is trusted) + SecurityHeaders
// with the given X-Forwarded-Proto on a request from that proxy.
func serveSecurityTrusted(xfp string) *httptest.ResponseRecorder {
	chain := Forwarded("192.0.2.1")(SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Forwarded-Proto", xfp)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	return rec
}

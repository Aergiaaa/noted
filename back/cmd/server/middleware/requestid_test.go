package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveRequestID runs the RequestID middleware with hdr (possibly empty)
// and returns the response header plus the id the handler saw in context.
func serveRequestID(t *testing.T, hdr string) (respHdr, ctxID string) {
	t.Helper()
	var seen string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	if hdr != "" {
		req.Header.Set(RequestIDHeader, hdr)
	}
	rec := httptest.NewRecorder()
	RequestID(next).ServeHTTP(rec, req)

	return rec.Header().Get(RequestIDHeader), seen
}

func TestRequestID_generatedWhenAbsentOrInvalid(t *testing.T) {
	for _, hdr := range []string{"", "has space", "%zz", strings.Repeat("a", 65)} {
		t.Run("input="+hdr, func(t *testing.T) {
			respHdr, ctxID := serveRequestID(t, hdr)

			if respHdr == "" {
				t.Fatal("response header = empty, want generated id")
			}
			if respHdr == hdr {
				t.Fatalf("response header = %q, want a fresh id (input rejected)", hdr)
			}
			if respHdr != ctxID {
				t.Fatalf("header = %q, context = %q, want equal", respHdr, ctxID)
			}
		})
	}
}

func TestRequestID_validHeader_echoed(t *testing.T) {
	for _, hdr := range []string{"abc-123_XYZ", "client-supplied-id", strings.Repeat("a", 64)} {
		t.Run(hdr, func(t *testing.T) {
			respHdr, ctxID := serveRequestID(t, hdr)

			if respHdr != hdr {
				t.Fatalf("response header = %q, want echoed %q", respHdr, hdr)
			}
			if ctxID != hdr {
				t.Fatalf("context id = %q, want %q", ctxID, hdr)
			}
		})
	}
}

func TestRequestIDFrom_withoutContext_empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if got := RequestIDFrom(req); got != "" {
		t.Fatalf("RequestIDFrom = %q, want empty", got)
	}
}

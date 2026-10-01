package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimit_smallBody_reachesHandlerIntact(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		got = string(b)
	})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"title":"x"}`))

	BodyLimit(next).ServeHTTP(httptest.NewRecorder(), req)

	if got != `{"title":"x"}` {
		t.Fatalf("body = %q, want original payload", got)
	}
}

func TestBodyLimit_overLimit_handlerGetsMaxBytesError(t *testing.T) {
	var readErr error
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	})
	payload := strings.Repeat("x", MaxBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))

	BodyLimit(next).ServeHTTP(httptest.NewRecorder(), req)

	var tooLarge *http.MaxBytesError
	if !errors.As(readErr, &tooLarge) {
		t.Fatalf("read error = %v, want *http.MaxBytesError", readErr)
	}
}

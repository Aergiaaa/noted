package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveRecover runs RequestID + Recover around next and returns the
// response plus the captured log (the rid comes from RequestID).
func serveRecover(t *testing.T, next http.HandlerFunc) (*httptest.ResponseRecorder, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	chain := RequestID(Recover(next))
	out := captureLog(t, func() { chain.ServeHTTP(rec, req) })
	return rec, out
}

func TestRecover_passthrough_writesThroughUnchanged(t *testing.T) {
	rec, out := serveRecover(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if out != "" {
		t.Fatalf("log = %q, want silent pass-through", out)
	}
}

func TestRecover_panic_writes500EnvelopeAndLogsPanic(t *testing.T) {
	rec, out := serveRecover(t, func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	})

	assertEnvelope(t, rec, http.StatusInternalServerError, "internal_error")

	rid := rec.Header().Get(REQUEST_ID_HEADER)
	for _, want := range []string{"panic rid=" + rid, "kaboom", "goroutine"} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, missing %q", out, want)
		}
	}
}

func TestRecover_errorPanic_writes500EnvelopeAndLogsCause(t *testing.T) {
	rec, out := serveRecover(t, func(http.ResponseWriter, *http.Request) {
		panic(errors.New("boom"))
	})

	assertEnvelope(t, rec, http.StatusInternalServerError, "internal_error")
	if !strings.Contains(out, "boom") {
		t.Fatalf("log = %q, missing panic cause", out)
	}
}

func TestRecover_errAbortHandler_repanicsWithoutStackLog(t *testing.T) {
	for _, tt := range []struct {
		name  string
		panic any
	}{
		{"bare", http.ErrAbortHandler},
		{"wrapped", fmt.Errorf("handler aborted: %w", http.ErrAbortHandler)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			old := log.Writer()
			log.SetOutput(&buf)
			defer log.SetOutput(old)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/boom", nil)
			chain := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				panic(tt.panic)
			}))

			var recovered any
			func() {
				defer func() { recovered = recover() }()
				chain.ServeHTTP(rec, req)
			}()

			err, ok := recovered.(error)
			if !ok || !errors.Is(err, http.ErrAbortHandler) {
				t.Fatalf("recovered = %#v, want ErrAbortHandler", recovered)
			}
			if buf.Len() != 0 {
				t.Fatalf("log = %q, want silent abort (no stack)", buf.String())
			}
		})
	}
}

func TestRecover_panicAfterWrite_abortsTruncatedResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	chain := Recover(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		panic("mid-write")
	}))

	var out string
	var recovered any
	out = captureLog(t, func() {
		func() {
			defer func() { recovered = recover() }()
			chain.ServeHTTP(rec, req)
		}()
	})

	err, ok := recovered.(error)
	if !ok || !errors.Is(err, http.ErrAbortHandler) {
		t.Fatalf("recovered = %#v, want ErrAbortHandler", recovered)
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d (envelope must not overwrite)", rec.Code, http.StatusTeapot)
	}
	for _, want := range []string{"panic", "mid-write", "goroutine"} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, missing %q", out, want)
		}
	}
}

func TestRecover_inChain_accessLogRecords500(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	chain := RequestID(AccessLog(Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("chain boom")
	}))))

	out := captureLog(t, func() { chain.ServeHTTP(rec, req) })

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	for _, want := range []string{"status=500", "chain boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, missing %q", out, want)
		}
	}
}

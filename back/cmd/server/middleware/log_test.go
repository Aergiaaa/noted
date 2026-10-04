package middleware

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLog swaps the stdlib logger for the duration of fn. Tests that
// touch process globals stay serial (TESTING.md), so no t.Parallel here.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	fn()
	return buf.String()
}

// serveAccessLog runs RequestID + AccessLog around next and returns the
// response plus the captured log line.
func serveAccessLog(t *testing.T, uri string, next http.HandlerFunc) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var out string
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, uri, nil)
	chain := RequestID(AccessLog(next))
	out = captureLog(t, func() { chain.ServeHTTP(rec, req) })
	return rec, out
}

func TestAccessLog_lineCarriesRequestIDMethodURIAndStatus(t *testing.T) {
	rec, line := serveAccessLog(t, "/healthz?x=1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	rid := rec.Header().Get(REQUEST_ID_HEADER)
	for _, want := range []string{
		"request ", "rid=" + rid, "method=GET", "uri=/healthz?x=1", "status=201", "dur=",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q missing %q", line, want)
		}
	}
}

func TestAccessLog_handlerWithoutWrite_logs200(t *testing.T) {
	_, line := serveAccessLog(t, "/noop", func(http.ResponseWriter, *http.Request) {})

	if !strings.Contains(line, "status=200") {
		t.Fatalf("log line %q missing default status 200", line)
	}
}

func TestAccessLog_writeWithoutExplicitHeader_logs200(t *testing.T) {
	_, line := serveAccessLog(t, "/implicit", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	if !strings.Contains(line, "status=200") {
		t.Fatalf("log line %q missing implicit status 200", line)
	}
}

func TestAccessLog_doubleWriteHeader_keepsFirstStatus(t *testing.T) {
	_, line := serveAccessLog(t, "/dup", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError)
	})

	if !strings.Contains(line, "status=201") {
		t.Fatalf("log line %q wants first status 201", line)
	}
}

func TestAccessLog_abortPanic_stillEmitsLine(t *testing.T) {
	// Recover's truncated-response path re-panics ErrAbortHandler; the
	// deferred emit must still log the request line on the way out.
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	chain := RequestID(AccessLog(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})))

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		chain.ServeHTTP(rec, req)
	}()

	err, ok := recovered.(error)
	if !ok || !errors.Is(err, http.ErrAbortHandler) {
		t.Fatalf("recovered = %#v, want ErrAbortHandler", recovered)
	}
	if line := buf.String(); !strings.Contains(line, "request ") || !strings.Contains(line, "uri=/abort") {
		t.Fatalf("log = %q, want request line despite abort", line)
	}
}

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const TEST_APP_ORIGIN = "http://localhost:5173"

// serveOrigin runs OriginCheck around a stub endpoint that reports
// whether the request was allowed through.
func serveOrigin(t *testing.T, method, uri string, mutate func(*http.Request)) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	var passed bool
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		passed = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(method, uri, nil)
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	OriginCheck(TEST_APP_ORIGIN)(next).ServeHTTP(rec, req)
	return rec, passed
}

func assertEnvelope(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d", rec.Code, status)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("code = %q, want %q", body.Error.Code, code)
	}
}

func TestOriginCheck_mutationWithForeignOrigin_403(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec, passed := serveOrigin(t, method, "/api/notes", func(r *http.Request) {
				r.Header.Set("Origin", "https://evil.example")
			})

			if passed {
				t.Fatal("request reached handler, want rejected")
			}
			assertEnvelope(t, rec, http.StatusForbidden, "origin_forbidden")
		})
	}
}

func TestOriginCheck_mutationWithTrustedOrigin_allowed(t *testing.T) {
	// Configured APP_ORIGIN, and the request's own host (same-origin
	// behind the dev/prod proxy).
	for _, origin := range []string{TEST_APP_ORIGIN, "http://example.com"} {
		t.Run(origin, func(t *testing.T) {
			rec, passed := serveOrigin(t, http.MethodPost, "/api/notes", func(r *http.Request) {
				r.Header.Set("Origin", origin)
			})

			if !passed || rec.Code != http.StatusNoContent {
				t.Fatalf("passed = %v status = %d, want allowed", passed, rec.Code)
			}
		})
	}
}

func TestOriginCheck_mutationWithoutOriginOrReferer_allowed(t *testing.T) {
	// Non-browser clients (curl) send neither header; SameSite=Lax plus
	// the browser's Origin requirement carry the CSRF weight instead.
	_, passed := serveOrigin(t, http.MethodPost, "/api/notes", nil)

	if !passed {
		t.Fatal("headerless mutation rejected, want allowed")
	}
}

func TestOriginCheck_refererCheckedWhenOriginMissing(t *testing.T) {
	t.Run("trusted referer allowed", func(t *testing.T) {
		_, passed := serveOrigin(t, http.MethodPost, "/api/notes", func(r *http.Request) {
			r.Header.Set("Referer", TEST_APP_ORIGIN+"/notes")
		})
		if !passed {
			t.Fatal("trusted referer rejected")
		}
	})
	t.Run("foreign referer rejected", func(t *testing.T) {
		rec, passed := serveOrigin(t, http.MethodPost, "/api/notes", func(r *http.Request) {
			r.Header.Set("Referer", "https://evil.example/x")
		})
		if passed {
			t.Fatal("foreign referer reached handler")
		}
		assertEnvelope(t, rec, http.StatusForbidden, "origin_forbidden")
	})
	t.Run("unparseable referer rejected", func(t *testing.T) {
		rec, passed := serveOrigin(t, http.MethodPost, "/api/notes", func(r *http.Request) {
			r.Header.Set("Referer", "http://[::1")
		})
		if passed {
			t.Fatal("unparseable referer reached handler")
		}
		assertEnvelope(t, rec, http.StatusForbidden, "origin_forbidden")
	})
}

func TestOriginCheck_unparseableOrigin_rejected(t *testing.T) {
	rec, passed := serveOrigin(t, http.MethodPost, "/api/notes", func(r *http.Request) {
		r.Header.Set("Origin", "%zz")
	})

	if passed {
		t.Fatal("unparseable origin reached handler")
	}
	assertEnvelope(t, rec, http.StatusForbidden, "origin_forbidden")
}

func TestOriginCheck_nonMutationMethods_skipCheck(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			_, passed := serveOrigin(t, method, "/api/notes", func(r *http.Request) {
				r.Header.Set("Origin", "https://evil.example")
			})

			if !passed {
				t.Fatalf("%s with foreign origin rejected, want skipped", method)
			}
		})
	}
}

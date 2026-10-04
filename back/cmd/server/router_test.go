package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- router ---

func TestRouter_unknownRoute_returns404Envelope(t *testing.T) {
	for _, path := range []string{"/nope", "/healthz/", "/api/healthz"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
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
			if body.Error.Code != "not_found" {
				t.Fatalf("code = %q, want not_found", body.Error.Code)
			}
		})
	}
}

func TestRouter_healthz_wrongMethod_returns405WithAllow(t *testing.T) {
	for _, method := range []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/healthz", nil)
			rec := httptest.NewRecorder()

			newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
			if rec.Header().Get("Allow") == "" {
				t.Fatal("Allow header missing, want chi's default 405 list")
			}
		})
	}
}

func TestRouter_healthz_queryParams_ignored(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz?foo=bar", nil)
	rec := httptest.NewRecorder()

	newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- middleware chain wiring ---

func TestRouter_responseHasRequestIDAndSecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("X-Request-ID missing, chain not wired")
	}
	if ct := rec.Header().Get("Content-Security-Policy"); !strings.Contains(ct, "frame-ancestors 'none'") {
		t.Fatalf("CSP = %q, want SECURITY.md policy", ct)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff header missing")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("X-Frame-Options missing")
	}
	if hsts := rec.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Fatalf("HSTS = %q, want unset on plain-HTTP dev", hsts)
	}
}

func TestRouter_echoesSuppliedRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "trace-me-42")
	rec := httptest.NewRecorder()

	newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "trace-me-42" {
		t.Fatalf("X-Request-ID = %q, want echoed trace-me-42", got)
	}
}

func TestRouter_mutationWithForeignOrigin_403BeforeRouting(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	newTestApp(Config{AppOrigin: "http://localhost:5173"}).newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (chain rejects before 405)", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if body.Error.Code != "origin_forbidden" {
		t.Fatalf("code = %q, want origin_forbidden", body.Error.Code)
	}
}

func TestRouter_mutationWithAppOrigin_reachesRouter(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	newTestApp(Config{AppOrigin: "http://localhost:5173"}).newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 (trusted origin passed the chain)", rec.Code)
	}
}

func TestRouter_accessLogLineEmitted(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

	line := buf.String()
	rid := rec.Header().Get("X-Request-ID")
	for _, want := range []string{"request ", "rid=" + rid, "method=GET", "uri=/healthz", "status=200"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q missing %q", line, want)
		}
	}
}

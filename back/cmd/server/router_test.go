package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"noted/internal/auth"
)

// --- router ---

func TestRouter_unknownRoute_returns404Envelope(t *testing.T) {
	for _, path := range []string{"/nope", "/healthz/", "/api/healthz"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

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

			newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

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

	newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- middleware chain wiring ---

func TestRouter_responseHasRequestIDAndSecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

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

	newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "trace-me-42" {
		t.Fatalf("X-Request-ID = %q, want echoed trace-me-42", got)
	}
}

func TestRouter_mutationWithForeignOrigin_403BeforeRouting(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	newTestApp(t, Config{AppOrigin: "http://localhost:5173"}).newRouter().ServeHTTP(rec, req)

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

	newTestApp(t, Config{AppOrigin: "http://localhost:5173"}).newRouter().ServeHTTP(rec, req)

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
	newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

	line := buf.String()
	rid := rec.Header().Get("X-Request-ID")
	for _, want := range []string{"request ", "rid=" + rid, "method=GET", "uri=/healthz", "status=200"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q missing %q", line, want)
		}
	}
}

// --- F4 auth routes ---

func TestRouter_apiSession_unauthenticated_401Envelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	rec := httptest.NewRecorder()

	newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if body.Error.Code != "unauthenticated" {
		t.Fatalf("code = %q, want unauthenticated", body.Error.Code)
	}
}

func TestRouter_guardedRoute_401WithoutSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/recovery-codes", nil)
	rec := httptest.NewRecorder()

	newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (RequireSession)", rec.Code)
	}
}

func TestRouter_logout_isPublic_204(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 without a session", rec.Code)
	}
}

func TestRouter_loginRateLimit_sixthAttempt_429(t *testing.T) {
	app := newTestApp(t, Config{})
	router := app.newRouter()

	attempt := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
			strings.NewReader(`{"code":"000000"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	for i := 1; i <= auth.LOGIN_RATE_LIMIT; i++ {
		if rec := attempt(); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401", i, rec.Code)
		}
	}
	rec := attempt()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 6 = %d, want 429", rec.Code)
	}
	if retry := rec.Header().Get("Retry-After"); retry == "" || retry == "0" {
		t.Fatalf("Retry-After = %q, want positive seconds", retry)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if body.Error.Code != "rate_limited" {
		t.Fatalf("code = %q, want rate_limited", body.Error.Code)
	}
}

func TestRouter_setupPaths_404NotAThing(t *testing.T) {
	// F4 retires SETUP_TOKEN: enrollment exists only as the host CLI, so
	// every HTTP setup surface must fall through to the 404 envelope.
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/auth/setup"},
		{http.MethodGet, "/api/auth/setup"},
		{http.MethodPost, "/api/setup"},
		{http.MethodPost, "/api/auth/enroll"},
		{http.MethodPost, "/api/auth/reset"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			newTestApp(t, Config{}).newRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
		})
	}
}

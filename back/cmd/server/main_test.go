package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// --- /healthz ---

func TestHealthz_returns200OkTrue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body["ok"] {
		t.Fatalf("body = %v, want {ok:true}", body)
	}
}

func TestHealthz_contentTypeAndExactBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}
	if got, want := rec.Body.String(), "{\"ok\":true}\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHealthz_wrongMethod_returns405(t *testing.T) {
	for _, method := range []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/healthz", nil)
			rec := httptest.NewRecorder()

			NewRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHealthz_unknownRoute_returns404(t *testing.T) {
	for _, path := range []string{"/nope", "/healthz/", "/api/healthz"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			NewRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
			}
		})
	}
}

func TestHealthz_queryParams_ignored(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz?foo=bar", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- envOr / configFromEnv ---

func TestEnvOr(t *testing.T) {
	const key = "NOTED_TEST_ENVOR"

	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	if got := envOr(key, "fallback"); got != "fallback" {
		t.Fatalf("unset: got %q, want %q", got, "fallback")
	}

	t.Setenv(key, "")
	if got := envOr(key, "fallback"); got != "fallback" {
		t.Fatalf("empty: got %q, want %q", got, "fallback")
	}

	t.Setenv(key, "value")
	if got := envOr(key, "fallback"); got != "value" {
		t.Fatalf("set: got %q, want %q", got, "value")
	}
}

func TestConfigFromEnv_defaults(t *testing.T) {
	// Empty string forces the fallback branch regardless of ambient env.
	for _, k := range []string{
		"APP_ENV", "APP_ORIGIN", "TZ", "DATA_PATH",
		"CURRENCY_DEFAULT", "SETUP_TOKEN", "TOTP_ENC_KEY",
		"TRUSTED_PROXIES", "ADDR",
	} {
		t.Setenv(k, "")
	}

	cfg := configFromEnv()

	want := Config{
		AppEnv: "dev", AppOrigin: "http://localhost:5173", TZ: "UTC",
		DataPath: "./.data/noted.db", CurrencyDefault: "USD",
		SetupToken: "", TOTPEncKey: "", TrustedProxies: "", Addr: ":8080",
	}
	if cfg != want {
		t.Fatalf("config = %+v, want %+v", cfg, want)
	}
}

func TestConfigFromEnv_overrides(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("APP_ORIGIN", "https://noted.example.com")
	t.Setenv("TZ", "Europe/Berlin")
	t.Setenv("DATA_PATH", "/data/noted.db")
	t.Setenv("CURRENCY_DEFAULT", "EUR")
	t.Setenv("SETUP_TOKEN", "setup-123")
	t.Setenv("TOTP_ENC_KEY", "enc-456")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8")
	t.Setenv("ADDR", "127.0.0.1:9090")

	cfg := configFromEnv()

	if cfg.AppEnv != "prod" || cfg.AppOrigin != "https://noted.example.com" ||
		cfg.TZ != "Europe/Berlin" || cfg.DataPath != "/data/noted.db" ||
		cfg.CurrencyDefault != "EUR" || cfg.SetupToken != "setup-123" ||
		cfg.TOTPEncKey != "enc-456" || cfg.TrustedProxies != "10.0.0.0/8" ||
		cfg.Addr != "127.0.0.1:9090" {
		t.Fatalf("config overrides not applied: %+v", cfg)
	}
}

// --- newServer ---

func TestNewServer_wiresAddrHandlerAndTimeout(t *testing.T) {
	cfg := Config{Addr: "127.0.0.1:8080"}
	srv := newServer(cfg)

	if srv.Addr != cfg.Addr {
		t.Fatalf("Addr = %q, want %q", srv.Addr, cfg.Addr)
	}
	if srv.Handler == nil {
		t.Fatal("Handler is nil")
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 5s", srv.ReadHeaderTimeout)
	}
}

// --- run ---

func waitForHealthz(t *testing.T, addr string) {
	t.Helper()
	url := fmt.Sprintf("http://%s/healthz", addr)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec,noctx // test-only local poll
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server on %s never became ready", addr)
}

func TestRun_gracefulShutdownOnCancel(t *testing.T) {
	cfg := Config{Addr: "127.0.0.1:18081"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx, cfg) }()

	waitForHealthz(t, cfg.Addr)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not shut down after cancel")
	}
}

func TestRun_bindError_returned(t *testing.T) {
	// Occupy a port so ListenAndServe fails deterministically.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	cfg := Config{Addr: ln.Addr().String()}
	if err := run(context.Background(), cfg); err == nil {
		t.Fatal("run with occupied addr returned nil, want bind error")
	}
}

func TestRun_shutdownTimeout_returned(t *testing.T) {
	old := shutdownTimeout
	shutdownTimeout = 300 * time.Millisecond // fail fast, don't wait 30s
	defer func() { shutdownTimeout = old }()

	// Distinct port per test.
	cfg := Config{Addr: "127.0.0.1:18082"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx, cfg) }()

	waitForHealthz(t, cfg.Addr)

	// Hold one connection mid-request (headers never terminate) so it
	// never goes idle and Shutdown hits the 300ms deadline.
	held, err := net.Dial("tcp", cfg.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Close() }()
	if _, err := fmt.Fprintf(held, "GET /healthz HTTP/1.1\r\nHost: %s\r\nX-Hold: 1\r\n", cfg.Addr); err != nil {
		t.Fatal(err)
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("run with stuck connection returned nil, want deadline error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}

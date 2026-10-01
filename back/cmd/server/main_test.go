package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noted/cmd/server/handler"
	"noted/internal/database"
)

// --- getEnv / newConfigFromEnv ---

func TestGetEnv(t *testing.T) {
	const KEY = "NOTED_TEST_GETENV"

	if err := os.Unsetenv(KEY); err != nil {
		t.Fatal(err)
	}
	if got := getEnv(KEY, "fallback"); got != "fallback" {
		t.Fatalf("unset: got %q, want %q", got, "fallback")
	}

	t.Setenv(KEY, "")
	if got := getEnv(KEY, "fallback"); got != "fallback" {
		t.Fatalf("empty: got %q, want %q", got, "fallback")
	}

	t.Setenv(KEY, "value")
	if got := getEnv(KEY, "fallback"); got != "value" {
		t.Fatalf("set: got %q, want %q", got, "value")
	}
}

func TestNewConfigFromEnv_defaults(t *testing.T) {
	// Empty string forces the fallback branch regardless of ambient env.
	for _, k := range []string{
		"APP_ENV", "APP_ORIGIN", "TZ", "DB_PATH",
		"CURRENCY_DEFAULT", "SETUP_TOKEN", "TOTP_ENC_KEY",
		"TRUSTED_PROXIES", "ADDR",
	} {
		t.Setenv(k, "")
	}

	cfg := newConfigFromEnv()

	want := Config{
		AppEnv: "dev", AppOrigin: "http://localhost:5173", TZ: "UTC",
		DBPath: "./.db/noted.db", CurrencyDefault: "USD",
		SetupToken: "", TOTPEncKey: "", TrustedProxies: "", Addr: ":8080",
	}
	if cfg != want {
		t.Fatalf("config = %+v, want %+v", cfg, want)
	}
}

func TestNewConfigFromEnv_overrides(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("APP_ORIGIN", "https://noted.example.com")
	t.Setenv("TZ", "Europe/Berlin")
	t.Setenv("DB_PATH", "/data/noted.db")
	t.Setenv("CURRENCY_DEFAULT", "EUR")
	t.Setenv("SETUP_TOKEN", "setup-123")
	t.Setenv("TOTP_ENC_KEY", "enc-456")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8")
	t.Setenv("ADDR", "127.0.0.1:9090")

	cfg := newConfigFromEnv()

	if cfg.AppEnv != "prod" || cfg.AppOrigin != "https://noted.example.com" ||
		cfg.TZ != "Europe/Berlin" || cfg.DBPath != "/data/noted.db" ||
		cfg.CurrencyDefault != "EUR" || cfg.SetupToken != "setup-123" ||
		cfg.TOTPEncKey != "enc-456" || cfg.TrustedProxies != "10.0.0.0/8" ||
		cfg.Addr != "127.0.0.1:9090" {
		t.Fatalf("config overrides not applied: %+v", cfg)
	}
}

func TestNewConfigFromEnv_staleDataPath_warns(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	t.Setenv("DATA_PATH", "/data/old.db")
	t.Setenv("DB_PATH", "")

	_ = newConfigFromEnv()

	if got := buf.String(); !strings.Contains(got, "DATA_PATH was renamed to DB_PATH") {
		t.Fatalf("log = %q, want stale DATA_PATH warning", got)
	}
}

// --- run ---

// testConfig gives run() an isolated temp DB so tests never touch .db/.
func testConfig(t *testing.T, addr string) Config {
	t.Helper()
	return Config{Addr: addr, DBPath: filepath.Join(t.TempDir(), "noted.db")}
}

// newTestApp wires a fresh handler layer so tests never repeat New().
func newTestApp(cfg Config) *app {
	return newApp(cfg, handler.New())
}

// freeAddr returns a 127.0.0.1 host:port that was free a moment ago, so
// tests don't collide on fixed ports (they stay serial anyway: they swap
// package-level shutdownTimeout / log output).
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

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
	cfg := testConfig(t, freeAddr(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- newTestApp(cfg).run(ctx) }()

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

	cfg := testConfig(t, ln.Addr().String())
	if err := newTestApp(cfg).run(context.Background()); err == nil {
		t.Fatal("run with occupied addr returned nil, want bind error")
	}
}

func TestRun_shutdownTimeout_returned(t *testing.T) {
	old := shutdownTimeout
	shutdownTimeout = 300 * time.Millisecond // fail fast, don't wait 25s
	defer func() { shutdownTimeout = old }()

	cfg := testConfig(t, freeAddr(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- newTestApp(cfg).run(ctx) }()

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

func TestRun_dbOpenError_returned(t *testing.T) {
	base := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(base, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Addr: freeAddr(t), DBPath: filepath.Join(base, "noted.db")}

	err := newTestApp(cfg).run(context.Background())
	if err == nil {
		t.Fatal("run with unopenable DB_PATH returned nil, want db error")
	}
	if !strings.Contains(err.Error(), "error opening db:") {
		t.Fatalf("error = %v, want \"error opening db:\" wrap", err)
	}
}

func TestRun_boot_migratesDataFile(t *testing.T) {
	cfg := testConfig(t, freeAddr(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- newTestApp(cfg).run(ctx) }()
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

	h, err := database.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("reopen migrated db: %v", err)
	}
	defer func() { _ = h.Close() }()

	var version int
	if err := h.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("user_version = %d, want 1 (boot migration ran)", version)
	}
}

// --- dbCloseWithErr ---

type stubCloser struct{ err error }

func (s *stubCloser) Close() error { return s.err }

func TestDBCloseWithErr_logsOnCloseError(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	dbCloseWithErr(&stubCloser{err: errors.New("boom")})

	if got := buf.String(); !strings.Contains(got, "error closing db: boom") {
		t.Fatalf("log = %q, want close error line", got)
	}
}

func TestDBCloseWithErr_silentOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	dbCloseWithErr(&stubCloser{})

	if buf.Len() != 0 {
		t.Fatalf("log = %q, want silent", buf.String())
	}
}

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

// Config holds env-driven settings. See .env.example / DEPLOYMENT.md.
// F1 only reads them; later features (auth, DB) will use them.
type Config struct {
	AppEnv          string
	AppOrigin       string
	TZ              string
	DataPath        string
	CurrencyDefault string
	SetupToken      string
	TOTPEncKey      string
	TrustedProxies  string
	Addr            string
}

func configFromEnv() Config {
	return Config{
		AppEnv:          envOr("APP_ENV", "dev"),
		AppOrigin:       envOr("APP_ORIGIN", "http://localhost:5173"),
		TZ:              envOr("TZ", "UTC"),
		DataPath:        envOr("DATA_PATH", "./.data/noted.db"),
		CurrencyDefault: envOr("CURRENCY_DEFAULT", "USD"),
		SetupToken:      os.Getenv("SETUP_TOKEN"),
		TOTPEncKey:      os.Getenv("TOTP_ENC_KEY"),
		TrustedProxies:  os.Getenv("TRUSTED_PROXIES"),
		Addr:            envOr("ADDR", ":8080"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// NewRouter builds the HTTP router. F1: only GET /healthz (no auth).
func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	return r
}

// newServer wires timeouts + handler in one place so run() and tests
// share the same constructor.
func newServer(cfg Config) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// shutdownTimeout bounds graceful shutdown. Package var (not const) so tests
// can force the timeout-error branch without waiting the full 30s.
var shutdownTimeout = 30 * time.Second

// run serves until ctx is cancelled or SIGINT/SIGTERM arrives, then shuts
// down gracefully. Startup (bind) and shutdown errors are returned so the
// caller — and tests — can observe them.
func run(ctx context.Context, cfg Config) error {
	srv := newServer(cfg)

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(sctx); err != nil {
			return err
		}
		return <-errCh
	}
}

// main is a thin entrypoint by design: all logic lives in run() so tests
// can cover every branch. func main itself is excluded from the 100%
// coverage gate (see Makefile test-cover-back) — it can only be exercised
// by booting the real binary.
func main() {
	if err := run(context.Background(), configFromEnv()); err != nil {
		log.Fatal(err)
	}
}

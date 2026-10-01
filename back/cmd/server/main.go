package main

import (
	"context"
	"fmt"
	"log"

	"noted/cmd/server/handler"
	"noted/internal/database"
)

// Config holds env-driven settings. See .env.example / DEPLOYMENT.md.
// DBPath is opened (and migrated) by run(); auth uses the rest in F4.
type Config struct {
	AppEnv          string
	AppOrigin       string
	TZ              string
	DBPath          string
	CurrencyDefault string
	SetupToken      string
	TOTPEncKey      string
	TrustedProxies  string
	Addr            string
}

// app carries the server's wiring (config + handler layer today; db and
// services in later features) so run() and the router share one receiver
// instead of passing Config through every call. Server lifecycle lives in
// server.go, routes in router.go; handlers live in the handler package,
// one file per endpoint.
type app struct {
	conf    Config
	handler *handler.Handler
}

// run opens (and migrates) the database, then serves until ctx is cancelled
// or SIGINT/SIGTERM arrives, shutting down gracefully. Startup (DB + bind)
// and shutdown errors are returned so the caller — and tests — can observe
// them.
func (a *app) run(ctx context.Context) error {
	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}
	defer dbCloseWithErr(db)

	if err := a.serve(ctx); err != nil {
		return fmt.Errorf("error serving app: %w", err)
	}

	return nil
}

// main is a thin entrypoint by design: all logic lives in run() so tests
// can cover every branch. func main itself is excluded from the 100%
// coverage gate (see Makefile test-cover-back) — it can only be exercised
// by booting the real binary.
func main() {
	conf := newConfigFromEnv()
	h := handler.New()

	a := newApp(conf, h)

	ctx := context.Background()
	if err := a.run(ctx); err != nil {
		log.Fatal(err)
	}
}

func newApp(conf Config, h *handler.Handler) *app {
	return &app{
		conf:    conf,
		handler: h,
	}
}

func newConfigFromEnv() Config {
	warnStaleDataPath()
	return Config{
		AppEnv:          getEnv("APP_ENV", "dev"),
		AppOrigin:       getEnv("APP_ORIGIN", "http://localhost:5173"),
		TZ:              getEnv("TZ", "UTC"),
		DBPath:          getEnv("DB_PATH", "./.db/noted.db"),
		CurrencyDefault: getEnv("CURRENCY_DEFAULT", "USD"),
		SetupToken:      getEnv("SETUP_TOKEN", ""),
		TOTPEncKey:      getEnv("TOTP_ENC_KEY", ""),
		TrustedProxies:  getEnv("TRUSTED_PROXIES", ""),
		Addr:            getEnv("ADDR", ":8080"),
	}
}

// dbCloser is the Close half of *sql.DB, extracted as an interface so tests
// can stub the error branch (sql.DB.Close does not fail on demand).
type dbCloser interface{ Close() error }

// dbCloseWithErr closes the database, logging — not returning — a close
// error: by the time the defer runs, run()'s result is already decided.
func dbCloseWithErr(db dbCloser) {
	if err := db.Close(); err != nil {
		log.Printf("error closing db: %v", err)
	}
}

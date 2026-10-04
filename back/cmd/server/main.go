package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"noted/cmd/server/handler"
	"noted/internal/auth"
	"noted/internal/database"
)

// Config holds env-driven settings. See .env.example / DEPLOYMENT.md.
// DBPath is opened (and migrated) by run(); TOTPEncKey feeds the auth
// service wired in wire().
type Config struct {
	AppEnv          string
	AppOrigin       string
	TZ              string
	DBPath          string
	CurrencyDefault string
	TOTPEncKey      string
	TrustedProxies  string
	Addr            string
}

// app carries the server's wiring (config + handler + auth) so run(),
// the router and the CLI share one receiver instead of passing Config
// through every call. Server lifecycle lives in server.go, routes in
// router.go, host commands in cli.go; handlers live in the handler
// package, one file per endpoint group.
type app struct {
	conf    Config
	handler *handler.Handler
	auth    *auth.Service
}

// run opens (and migrates) the database, wires the auth service and
// handler over it, then serves until ctx is cancelled or SIGINT/SIGTERM
// arrives, shutting down gracefully. Startup (DB + bind) and shutdown
// errors are returned so the caller — and tests — can observe them.
func (a *app) run(ctx context.Context) error {
	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}
	defer dbCloseWithErr(db)

	a.wire(db)

	if err := a.serve(ctx); err != nil {
		return fmt.Errorf("error serving app: %w", err)
	}

	return nil
}

// wire builds the auth service and the handler over an open database and
// runs the auth housekeeping that belongs to boot: expired-session pruning
// and the enrollment warning (SECURITY.md: first boot is `server enroll`).
// Split from run() so tests can wire an existing handle directly.
func (a *app) wire(db *sql.DB) {
	a.auth = auth.New(db, a.conf.TOTPEncKey)
	a.handler = handler.New(a.auth, a.conf.AppEnv == "prod")
	a.bootAuth(context.Background())
}

// bootAuth prunes sessions whose expiry has passed and warns — loudly,
// once — while the host has no TOTP enrollment: nobody can log in yet.
// Failures log instead of blocking boot; a broken DB fails the first
// request loudly anyway.
func (a *app) bootAuth(ctx context.Context) {
	if err := a.auth.PruneSessions(ctx); err != nil {
		log.Printf("error pruning sessions: %v", err)
	}
	enrolled, err := a.auth.Enrolled(ctx)
	switch {
	case err != nil:
		log.Printf("error checking enrollment: %v", err)
	case !enrolled:
		log.Printf("WARNING: not enrolled — run `server enroll` (make enroll) before anyone can log in")
	}
}

// main is a thin entrypoint by design: config, then the host CLI (it
// either handles the argv and exits, or falls through to serving), then
// run() so tests can cover every branch. func main itself is excluded
// from the 100% coverage gate (see Makefile test-cover-back).
func main() {
	conf := newConfigFromEnv()
	a := newApp(conf)

	if handled, code := a.cli(os.Args[1:]); handled {
		os.Exit(code)
	}

	ctx := context.Background()
	if err := a.run(ctx); err != nil {
		log.Fatal(err)
	}
}

func newApp(conf Config) *app {
	return &app{conf: conf}
}

func newConfigFromEnv() Config {
	warnStaleDataPath()
	return Config{
		AppEnv:          getEnv("APP_ENV", "dev"),
		AppOrigin:       getEnv("APP_ORIGIN", "http://localhost:5173"),
		TZ:              getEnv("TZ", "UTC"),
		DBPath:          getEnv("DB_PATH", "./.db/noted.db"),
		CurrencyDefault: getEnv("CURRENCY_DEFAULT", "USD"),
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

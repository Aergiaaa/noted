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

// Config holds env-driven settings (.env.example, DEPLOYMENT.md).
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

// app carries the wiring so run(), the router and the CLI share one
// receiver instead of passing Config through every call.
type app struct {
	conf    Config
	handler *handler.Handler
	auth    *auth.Service

	// secure pins the cookie Secure attribute to APP_ENV=prod; derived
	// once in wire so the login cookie and the guard's re-issue can never
	// disagree.
	secure bool
}

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

// main stays a thin entrypoint so run()'s branches stay testable; func main
// itself is excluded from the 100% coverage gate (Makefile test-cover-back).
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

// wire is split from run() so tests can wire an existing handle directly.
func (a *app) wire(db *sql.DB) {
	a.auth = auth.New(db, a.conf.TOTPEncKey)
	a.secure = a.conf.AppEnv == "prod"

	a.handler = handler.New(a.auth, a.secure)
	a.bootAuth(context.Background())
}

// bootAuth failures log instead of blocking boot — a broken DB fails the
// first request loudly anyway. The not-enrolled warning fires once per boot.
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

// dbCloser is the Close half of *sql.DB, extracted so tests can stub the
// error branch — sql.DB.Close does not fail on demand.
type dbCloser interface{ Close() error }

// dbCloseWithErr logs rather than returns: by the time this defer runs,
// run()'s result is already decided.
func dbCloseWithErr(db dbCloser) {
	if err := db.Close(); err != nil {
		log.Printf("error closing db: %v", err)
	}
}

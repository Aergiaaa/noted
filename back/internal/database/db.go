package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go driver (CGO off); registers "sqlite"
)

// driverName is a var so tests can force sql.Open's unknown-driver error
// branch (same trick as server.go's shutdownTimeout).
var driverName = "sqlite"

// Only the owner may reach the database or its directory.
const (
	DIR_PERM  os.FileMode = 0o700
	FILE_PERM os.FileMode = 0o600
)

// dsn: pragmas are per-connection DSN params, not one-shot PRAGMA
// statements, so every pooled connection gets them. _txlock=immediate
// takes the write lock up front, so a multi-statement write can never
// deadlock upgrading from a deferred read transaction (DATABASE.md). The
// shorthand keys are driver-validated, so a typo fails loudly.
func dsn(path string) string {
	q := url.Values{}
	q.Set("_journal_mode", "WAL")
	q.Set("_busy_timeout", "5000")
	q.Set("_foreign_keys", "on")
	q.Set("_synchronous", "normal")
	q.Set("_txlock", "immediate")
	return path + "?" + q.Encode()
}

func Open(path string) (*sql.DB, error) {
	if err := prepare(path); err != nil {
		return nil, err
	}
	h, err := sql.Open(driverName, dsn(path))
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	if err := h.Ping(); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	if err := Migrate(h); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return h, nil
}

// prepare creates the directory (0700) and file (0600), tightening a
// pre-existing regular file. Pre-existing directories stay as the operator
// made them — DB_PATH may point into an already-correct volume, and chmod
// could fail on a read-only mount. Anything else at path errors instead of
// being chmod-ed: tightening a directory to 0600 drops its x bit, so the
// boot would stay broken after the operator fixes the config.
func prepare(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DIR_PERM); err != nil {
		return fmt.Errorf("db dir %s: %w", dir, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL, FILE_PERM)
	switch {
	case err == nil:
		err = f.Close()
	case errors.Is(err, fs.ErrExist):
		err = tightenExisting(path)
	}
	if err != nil {
		return fmt.Errorf("db file %s: %w", path, err)
	}
	return nil
}

// tightenExisting follows symlinks (os.Stat), so a symlink to the database
// tightens the real file — the one SQLite opens — while a symlink to a
// directory fails the regular-file check untouched.
func tightenExisting(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("not a regular file (%s)", fi.Mode())
	}
	return os.Chmod(path, FILE_PERM)
}

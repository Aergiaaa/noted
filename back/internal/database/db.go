// Package database owns the SQLite connection: pragmas, file permissions, and the
// embedded migration runner. Services take sqlc's *generated.Queries built
// from a handle opened here; nothing else opens connections.
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

// driverName is a var rather than a const so tests can force sql.Open's
// unknown-driver error branch (same trick as server.go's shutdownTimeout).
var driverName = "sqlite"

// DIR_PERM/FILE_PERM: only the owner may reach the database or its directory.
// MkdirAll applies DIR_PERM on creation; an existing DB file is tightened to
// FILE_PERM in prepare.
const (
	DIR_PERM  os.FileMode = 0o700
	FILE_PERM os.FileMode = 0o600
)

// dsn builds the SQLite data source name. Pragmas are per-connection DSN
// params (not one-shot PRAGMA statements) so every pooled connection gets
// them: WAL journal, 5s busy timeout for the single-writer setup, foreign
// keys ON, NORMAL sync (DATABASE.md). The shorthand keys are validated by
// the driver, so a typo fails loudly instead of being ignored.
func dsn(path string) string {
	q := url.Values{}
	q.Set("_journal_mode", "WAL")
	q.Set("_busy_timeout", "5000")
	q.Set("_foreign_keys", "on")
	q.Set("_synchronous", "normal")
	return path + "?" + q.Encode()
}

// Open prepares the filesystem, opens the database at path, applies pending
// migrations, and returns a ready handle. The caller closes it.
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

// prepare makes the parent directory (0700 when created) and the database
// file (0600), tightening a pre-existing file. Pre-existing directories are
// left as the operator made them: DB_PATH may point into a volume that is
// already correct, and chmod-ing it could fail on a read-only mount.
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
		err = os.Chmod(path, FILE_PERM)
	}
	if err != nil {
		return fmt.Errorf("db file %s: %w", path, err)
	}
	return nil
}

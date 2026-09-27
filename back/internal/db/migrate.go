package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsDir is the embed path loadMigrations reads. A var (not const)
// only so tests can point Migrate at a missing directory; the embed pattern
// above stays the literal "migrations/*.sql".
var migrationsDir = "migrations"

// migrationNameRE matches the append-only migration files: NNNN_snake_case.sql
// (DEVELOPMENT.md conventions).
var migrationNameRE = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

// migration is one parsed migration file: version taken from the filename,
// body verbatim from the embedded file.
type migration struct {
	version int
	name    string
	body    string
}

// Migrate applies every embedded migration with a version newer than the
// database's PRAGMA user_version, oldest first. Each migration and its
// version bump share one transaction, so a failure leaves the database
// exactly as it was. Running Migrate twice is a no-op.
func Migrate(h *sql.DB) error {
	ms, err := loadMigrations(migrationsFS, migrationsDir)
	if err != nil {
		return err
	}
	var version int
	if err := h.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	for _, m := range ms {
		if m.version <= version {
			continue
		}
		if err := apply(h, m); err != nil {
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
	}
	return nil
}

// apply runs one migration body plus its version bump inside a single
// transaction. The pragma rides in the same statement so SQLite cannot
// record the version without the schema change (and vice versa).
func apply(h *sql.DB, m migration) error {
	tx, err := h.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	stmt := m.body + "\nPRAGMA user_version = " + strconv.Itoa(m.version) + ";"
	if _, err := tx.Exec(stmt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// loadMigrations parses dir inside fsys, ordered by version. A malformed
// filename, unreadable file, or duplicate version is a hard error: a typo
// must never silently skip a migration.
func loadMigrations(fsys fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	ms := make([]migration, 0, len(entries))
	for _, e := range entries {
		g := migrationNameRE.FindStringSubmatch(e.Name())
		if g == nil {
			return nil, fmt.Errorf("migration %q: want NNNN_name.sql", e.Name())
		}
		// Atoi cannot fail here: the regexp guarantees four ASCII digits.
		v, _ := strconv.Atoi(g[1])
		body, err := fs.ReadFile(fsys, filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		ms = append(ms, migration{version: v, name: e.Name(), body: string(body)})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	for i := 1; i < len(ms); i++ {
		if ms[i].version == ms[i-1].version {
			return nil, fmt.Errorf("duplicate migration version %04d", ms[i].version)
		}
	}
	return ms, nil
}

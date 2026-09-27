package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// freshPath returns a DB path in a dedicated temp dir (never the prod file).
func freshPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "data", "noted.db")
}

func userVersion(t *testing.T, h *sql.DB) int {
	t.Helper()
	var v int
	if err := h.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return v
}

func wantPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Fatalf("%s perm = %04o, want %04o", path, got, want)
	}
}

// --- dsn ---

func TestDsn_carriesAllPragmas(t *testing.T) {
	got := dsn("/data/noted.db")
	for _, want := range []string{
		"/data/noted.db?",
		"_journal_mode=WAL",
		"_busy_timeout=5000",
		"_foreign_keys=on",
		"_synchronous=normal",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("dsn %q missing %q", got, want)
		}
	}
}

// --- Open ---

func TestOpen_freshDb_migratesWithPragmasAndPerms(t *testing.T) {
	path := freshPath(t)

	h, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	wantPerm(t, filepath.Dir(path), 0o700)
	wantPerm(t, path, 0o600)

	var journal string
	if err := h.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journal)
	}
	var busy, fk int
	if err := h.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if busy != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busy)
	}
	if err := h.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fk)
	}
	if v := userVersion(t, h); v != 1 {
		t.Fatalf("user_version = %d, want 1", v)
	}
	var tables int
	if err := h.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN
		 ('notes','events','tasks','transactions','sessions','recovery_codes')`,
	).Scan(&tables); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tables != 6 {
		t.Fatalf("tables created = %d, want 6", tables)
	}
}

func TestOpen_existingFile_tightenedTo0600(t *testing.T) {
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	wantPerm(t, path, 0o600)
	if v := userVersion(t, h); v != 1 {
		t.Fatalf("user_version = %d, want 1 (migrations still applied)", v)
	}
}

func TestOpen_parentIsFile_dirError(t *testing.T) {
	base := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(base, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Open(filepath.Join(base, "noted.db"))
	if err == nil {
		t.Fatal("Open under a file returned nil, want dir error")
	}
	if !strings.Contains(err.Error(), "db dir") {
		t.Fatalf("error = %v, want db dir error", err)
	}
}

func TestOpen_invalidFileName_fileError(t *testing.T) {
	// NUL in the path: MkdirAll only sees the parent, OpenFile fails EINVAL.
	_, err := Open(filepath.Join(t.TempDir(), "bad\x00name.db"))
	if err == nil {
		t.Fatal("Open with NUL in filename returned nil, want file error")
	}
	if !strings.Contains(err.Error(), "db file") {
		t.Fatalf("error = %v, want db file error", err)
	}
}

func TestOpen_unknownDriver_error(t *testing.T) {
	old := driverName
	driverName = "noted-bogus-driver"
	t.Cleanup(func() { driverName = old })

	_, err := Open(filepath.Join(t.TempDir(), "noted.db"))
	if err == nil {
		t.Fatal("Open with unknown driver returned nil, want error")
	}
	if !strings.Contains(err.Error(), "sqlite open") {
		t.Fatalf("error = %v, want sqlite open error", err)
	}
}

func TestOpen_pathIsDirectory_pingError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asdir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	// prepare tightens the existing entry to 0600; restore so t.TempDir
	// cleanup can walk it.
	t.Cleanup(func() { _ = os.Chmod(path, 0o700) })

	_, err := Open(path)
	if err == nil {
		t.Fatal("Open on a directory returned nil, want ping error")
	}
	if !strings.Contains(err.Error(), "ping") {
		t.Fatalf("error = %v, want ping error", err)
	}
}

func TestOpen_migrationFailure_returned(t *testing.T) {
	path := freshPath(t)

	// Build a DB that already has the schema but claims version 0, so the
	// next Open re-runs 0001_init and trips over its own tables.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	h, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := Migrate(h); err != nil {
		t.Fatalf("seed migrate: %v", err)
	}
	if _, err := h.Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatalf("reset version: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = Open(path)
	if err == nil {
		t.Fatal("Open with conflicting schema returned nil, want migrate error")
	}
	if !strings.Contains(err.Error(), "migrate") {
		t.Fatalf("error = %v, want migrate error", err)
	}
}

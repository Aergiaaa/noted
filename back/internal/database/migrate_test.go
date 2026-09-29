package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func openBare(t *testing.T) *sql.DB {
	t.Helper()
	h, err := sql.Open(driverName, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// --- loadMigrations ---

func TestLoadMigrations_embeddedInitFound(t *testing.T) {
	ms, err := loadMigrations(migrationsFS, migrationsDir)
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("len = %d, want 1", len(ms))
	}
	if ms[0].version != 1 || ms[0].name != "0001_init.sql" {
		t.Fatalf("got %+v, want version 1 / 0001_init.sql", ms[0])
	}
	if !strings.Contains(ms[0].body, "CREATE TABLE notes") {
		t.Fatalf("body missing schema:\n%s", ms[0].body)
	}
}

func TestLoadMigrations_missingDir_returned(t *testing.T) {
	_, err := loadMigrations(fstest.MapFS{}, "migrations")
	if err == nil {
		t.Fatal("missing dir returned nil, want error")
	}
	if !strings.Contains(err.Error(), "read migrations") {
		t.Fatalf("error = %v, want read dir error", err)
	}
}

func TestLoadMigrations_malformedName_returned(t *testing.T) {
	fsys := fstest.MapFS{"migrations/notes.sql": {Data: []byte("SELECT 1")}}
	_, err := loadMigrations(fsys, "migrations")
	if err == nil {
		t.Fatal("malformed name returned nil, want error")
	}
	if !strings.Contains(err.Error(), "NNNN_name.sql") {
		t.Fatalf("error = %v, want filename format error", err)
	}
}

func TestLoadMigrations_unreadableFile_returned(t *testing.T) {
	// A directory wearing a migration filename: matching name, unreadable body.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "0002_stuck.sql"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := loadMigrations(os.DirFS(dir), ".")
	if err == nil {
		t.Fatal("unreadable migration returned nil, want error")
	}
	if !strings.Contains(err.Error(), "read 0002_stuck.sql") {
		t.Fatalf("error = %v, want read file error", err)
	}
}

func TestLoadMigrations_duplicateVersion_returned(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/0001_a.sql": {Data: []byte("SELECT 1")},
		"migrations/0001_b.sql": {Data: []byte("SELECT 1")},
	}
	_, err := loadMigrations(fsys, "migrations")
	if err == nil {
		t.Fatal("duplicate version returned nil, want error")
	}
	if !strings.Contains(err.Error(), "duplicate migration version 0001") {
		t.Fatalf("error = %v, want duplicate version error", err)
	}
}

// --- Migrate / apply ---

func TestMigrate_freshDb_appliesCleanAndIsIdempotent(t *testing.T) {
	h := openBare(t)

	if err := Migrate(h); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if v := userVersion(t, h); v != 1 {
		t.Fatalf("user_version = %d, want 1", v)
	}

	if err := Migrate(h); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if v := userVersion(t, h); v != 1 {
		t.Fatalf("user_version after re-run = %d, want 1", v)
	}
}

func TestMigrate_closedDb_userVersionReadError(t *testing.T) {
	h := openBare(t)
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}

	err := Migrate(h)
	if err == nil {
		t.Fatal("Migrate on closed db returned nil, want error")
	}
	if !strings.Contains(err.Error(), "read user_version") {
		t.Fatalf("error = %v, want user_version read error", err)
	}
}

func TestMigrate_missingMigrationsDir_returned(t *testing.T) {
	old := migrationsDir
	migrationsDir = "no-such-dir"
	t.Cleanup(func() { migrationsDir = old })

	err := Migrate(openBare(t))
	if err == nil {
		t.Fatal("Migrate without migrations returned nil, want error")
	}
	if !strings.Contains(err.Error(), "read no-such-dir") {
		t.Fatalf("error = %v, want read dir error", err)
	}
}

func TestApply_closedDb_beginError(t *testing.T) {
	h := openBare(t)
	ms, err := loadMigrations(migrationsFS, migrationsDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}

	err = apply(h, ms[0])
	if err == nil {
		t.Fatal("apply on closed db returned nil, want error")
	}
	if !strings.Contains(err.Error(), "begin") {
		t.Fatalf("error = %v, want begin error", err)
	}
}

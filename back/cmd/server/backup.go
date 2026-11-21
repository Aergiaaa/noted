package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"noted/internal/database"
)

// Nightly backup (DEPLOYMENT.md): one dated file, integrity-checked before
// it counts, 14 kept. BACKUP_KEEP is a constant because a fourteen-day
// window is the documented policy, not a per-run knob.
const (
	DEFAULT_BACKUP_DIR = "/backups"
	BACKUP_KEEP        = 14
	BACKUP_DATE_FORMAT = "2006-01-02"
	BACKUP_NAME_PREFIX = "noted-"
	BACKUP_NAME_SUFFIX = ".db"
)

// Two seams for branches a real run cannot reach: a fresh VACUUM output is
// always valid and always chmod-able, so the failure paths would otherwise
// be uncovered (same trick as database.driverName). Tests that swap them
// must stay serial.
var (
	verifyBackup   = integrityOK
	chmodBackup    = os.Chmod
	pruneBackupsFn = pruneBackups
)

// runBackup writes one dated backup (`--dir`, default /backups) or verifies
// an existing file (`--verify FILE`, the restore drill).
func (a *app) runBackup(args []string) int {
	dir := DEFAULT_BACKUP_DIR
	dirSet := false
	verifyPath := ""
	verifySet := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 >= len(args) {
				return backupUsage()
			}
			i++
			dir, dirSet = args[i], true
		case "--verify":
			if i+1 >= len(args) {
				return backupUsage()
			}
			i++
			verifyPath, verifySet = args[i], true
		default:
			return backupUsage()
		}
	}

	if verifySet && dirSet {
		return backupUsage()
	}

	if verifySet {
		if err := integrityOK(verifyPath); err != nil {
			fmt.Fprintf(os.Stderr, "backup: verify %s: %v\n", verifyPath, err)
			return 1
		}
		fmt.Printf("integrity: ok — %s\n", verifyPath)
		return 0
	}

	return a.writeBackup(dir)
}

func backupUsage() int {
	fmt.Fprintln(os.Stderr, "usage: server backup [--dir DIR] | server backup --verify FILE")
	return 2
}

func (a *app) writeBackup(dir string) int {
	if err := os.MkdirAll(dir, database.DIR_PERM); err != nil {
		fmt.Fprintf(os.Stderr, "backup: dir %s: %v\n", dir, err)
		return 1
	}

	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backup: %v\n", err)
		return 1
	}
	defer func() { _ = db.Close() }()

	target := filepath.Join(dir,
		BACKUP_NAME_PREFIX+a.now().UTC().Format(BACKUP_DATE_FORMAT)+BACKUP_NAME_SUFFIX)
	// A same-day rerun replaces its namesake: VACUUM INTO refuses an
	// existing target. A removal failure surfaces from VACUUM below.
	_ = os.Remove(target)

	if err := vacuumInto(db, target); err != nil {
		fmt.Fprintf(os.Stderr, "backup: vacuum into %s: %v\n", target, err)
		return 1
	}

	if err := chmodBackup(target, database.FILE_PERM); err != nil {
		fmt.Fprintf(os.Stderr, "backup: chmod %s: %v\n", target, err)
		return 1
	}

	if err := verifyBackup(target); err != nil {
		fmt.Fprintf(os.Stderr, "backup: verify %s: %v\n", target, err)
		return 1
	}

	kept, err := pruneBackupsFn(dir, BACKUP_KEEP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backup: prune %s: %v\n", dir, err)
		return 1
	}

	fmt.Printf("backup: %s (integrity ok, %d kept)\n", target, kept)
	return 0
}

// vacuumInto compacts the live database into a new file. Neither VACUUM nor
// PRAGMA accepts bind parameters, so the path is quoted by hand — it comes
// from the operator's CLI, never from a request.
func vacuumInto(db *sql.DB, path string) error {
	escaped := strings.ReplaceAll(path, "'", "''")
	_, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped))
	return err
}

// integrityOK opens a backup read-only (never migrate a file we are only
// checking) and requires SQLite's full integrity check to say "ok".
func integrityOK(path string) error {
	// sql.Open only fails for an unknown driver; internal/database's blank
	// import registers "sqlite" for the whole binary.
	h, _ := sql.Open("sqlite", "file:"+path+"?mode=ro")
	defer func() { _ = h.Close() }()

	var result string
	if err := h.QueryRow(integrityQuery).Scan(&result); err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}

	if result != "ok" {
		return fmt.Errorf("integrity_check: %s", result)
	}

	return nil
}

// integrityQuery is a seam so tests can exercise the non-ok text branch; a
// genuinely corrupt file normally fails the scan instead.
var integrityQuery = "PRAGMA integrity_check"

// pruneBackups keeps the newest `keep` dated files and returns how many
// remain. Names sort chronologically (YYYY-MM-DD), so no stat calls.
func pruneBackups(dir string, keep int) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() ||
			!strings.HasPrefix(name, BACKUP_NAME_PREFIX) ||
			!strings.HasSuffix(name, BACKUP_NAME_SUFFIX) {
			continue
		}
		names = append(names, name)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	survivors := min(keep, len(names))
	for _, name := range names[survivors:] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return 0, err
		}
	}

	return survivors, nil
}

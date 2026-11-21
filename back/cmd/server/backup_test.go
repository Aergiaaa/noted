package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noted/internal/database"
)

// fixedDay pins the dated filename so tests can assert it.
var fixedDay = time.Date(2026, 11, 20, 10, 30, 0, 0, time.UTC)

func backupApp(t *testing.T) *app {
	t.Helper()
	a := newCLIApp(t)
	a.now = func() time.Time { return fixedDay }
	return a
}

// backupDir creates a directory isolated to one test.
func backupDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "backups")
}

func backupFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// chmodForTest relaxes a mode again before t.TempDir's cleanup removes it.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o700) })
}

func TestBackup_writesDatedFileWithIntegrityAndMode(t *testing.T) {
	a := backupApp(t)
	dir := backupDir(t)

	out, errOut, code := runCLI(t, a, []string{"backup", "--dir", dir})
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}

	target := filepath.Join(dir, "noted-2026-11-20.db")
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	if mode := info.Mode().Perm(); mode != database.FILE_PERM {
		t.Fatalf("mode = %o, want %o", mode, database.FILE_PERM)
	}
	if err := integrityOK(target); err != nil {
		t.Fatalf("backup not valid: %v", err)
	}
	if !strings.Contains(out, target) || !strings.Contains(out, "integrity ok") {
		t.Fatalf("stdout = %q, want target + integrity ok", out)
	}
}

func TestBackup_sameDayRerunReplaces(t *testing.T) {
	a := backupApp(t)
	dir := backupDir(t)

	for run := 1; run <= 2; run++ {
		if _, errOut, code := runCLI(t, a, []string{"backup", "--dir", dir}); code != 0 {
			t.Fatalf("run %d: exit != 0 (stderr: %s)", run, errOut)
		}
	}
	if names := backupFiles(t, dir); len(names) != 1 {
		t.Fatalf("files = %v, want exactly one same-day backup", names)
	}
}

func TestBackup_prunesToKeep(t *testing.T) {
	a := backupApp(t)
	dir := backupDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 20; i++ {
		name := fmt.Sprintf("noted-2020-01-%02d.db", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	out, errOut, code := runCLI(t, a, []string{"backup", "--dir", dir})
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}

	names := backupFiles(t, dir)
	if len(names) != BACKUP_KEEP {
		t.Fatalf("kept %d files, want %d (files %v)", len(names), BACKUP_KEEP, names)
	}
	if _, err := os.Stat(filepath.Join(dir, "noted-2020-01-07.db")); err == nil {
		t.Fatal("oldest file survived pruning")
	}
	if _, err := os.Stat(filepath.Join(dir, "noted-2020-01-08.db")); err != nil {
		t.Fatalf("newest of the old files was pruned: %v", err)
	}
	if !strings.Contains(out, fmt.Sprintf("%d kept", BACKUP_KEEP)) {
		t.Fatalf("stdout = %q, want kept count", out)
	}
}

func TestBackup_pruneKeepsForeignFiles(t *testing.T) {
	a := backupApp(t)
	dir := backupDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "noted-2020-01-01.db"), 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(foreign, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, errOut, code := runCLI(t, a, []string{"backup", "--dir", dir}); code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign file removed: %v", err)
	}
}

func TestBackup_dirIsFile_exits1(t *testing.T) {
	a := backupApp(t)
	dir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(dir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, errOut, code := runCLI(t, a, []string{"backup", "--dir", dir})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "backup: dir") {
		t.Fatalf("stderr = %q, want dir error", errOut)
	}
}

func TestBackup_dbOpenError_exits1(t *testing.T) {
	a := backupApp(t)
	a.conf.DBPath = t.TempDir() // a directory can never open as SQLite

	_, errOut, code := runCLI(t, a, []string{"backup", "--dir", backupDir(t)})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "backup:") {
		t.Fatalf("stderr = %q, want open error", errOut)
	}
}

func TestBackup_vacuumError_exits1(t *testing.T) {
	a := backupApp(t)
	dir := backupDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	chmodForTest(t, dir, 0o500) // readable, not writable

	_, errOut, code := runCLI(t, a, []string{"backup", "--dir", dir})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "vacuum") {
		t.Fatalf("stderr = %q, want vacuum error", errOut)
	}
}

func TestBackup_chmodError_exits1(t *testing.T) {
	old := chmodBackup
	chmodBackup = func(string, os.FileMode) error { return fmt.Errorf("denied") }
	t.Cleanup(func() { chmodBackup = old })

	_, errOut, code := runCLI(t, backupApp(t), []string{"backup", "--dir", backupDir(t)})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "chmod") {
		t.Fatalf("stderr = %q, want chmod error", errOut)
	}
}

func TestBackup_verifyError_exits1(t *testing.T) {
	old := verifyBackup
	verifyBackup = func(string) error { return fmt.Errorf("malformed") }
	t.Cleanup(func() { verifyBackup = old })

	_, errOut, code := runCLI(t, backupApp(t), []string{"backup", "--dir", backupDir(t)})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "verify") {
		t.Fatalf("stderr = %q, want verify error", errOut)
	}
}

func TestBackup_badUsage_exits2(t *testing.T) {
	a := backupApp(t)
	for _, args := range [][]string{
		{"backup", "--dir"},
		{"backup", "--verify"},
		{"backup", "--nope"},
		{"backup", "--dir", "a", "--verify", "b"},
	} {
		if _, errOut, code := runCLI(t, a, args); code != 2 {
			t.Errorf("args %v: exit = %d, want 2 (stderr %q)", args, code, errOut)
		}
	}
}

func TestBackup_verify_ok(t *testing.T) {
	a := backupApp(t)
	dir := backupDir(t)
	if _, _, code := runCLI(t, a, []string{"backup", "--dir", dir}); code != 0 {
		t.Fatal("setup backup failed")
	}

	out, errOut, code := runCLI(t, a, []string{
		"backup", "--verify", filepath.Join(dir, "noted-2026-11-20.db"),
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "integrity: ok") {
		t.Fatalf("stdout = %q, want integrity ok", out)
	}
}

func TestBackup_verify_missingFile_exits1(t *testing.T) {
	_, errOut, code := runCLI(t, backupApp(t), []string{
		"backup", "--verify", filepath.Join(t.TempDir(), "absent.db"),
	})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "verify") {
		t.Fatalf("stderr = %q, want verify error", errOut)
	}
}

func TestBackup_verify_corruptFile_exits1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(path, []byte("definitely not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, errOut, code := runCLI(t, backupApp(t), []string{"backup", "--verify", path})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "integrity_check") {
		t.Fatalf("stderr = %q, want integrity_check error", errOut)
	}
}

func TestBackup_verify_nonOkResult_exits1(t *testing.T) {
	// A SQLite integrity_check reports problems as rows, not errors; the
	// seam makes that branch reachable with a real readable file.
	old := integrityQuery
	integrityQuery = "SELECT 'malformed page in main'"
	t.Cleanup(func() { integrityQuery = old })

	path := filepath.Join(t.TempDir(), "valid.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	_, errOut, code := runCLI(t, backupApp(t), []string{"backup", "--verify", path})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "malformed page") {
		t.Fatalf("stderr = %q, want the check's complaint", errOut)
	}
}

func TestPruneBackups_readDirError(t *testing.T) {
	dir := t.TempDir()
	chmodForTest(t, dir, 0o300) // executable, not readable

	if _, err := pruneBackups(dir, 1); err == nil {
		t.Fatal("pruneBackups on unreadable dir: want error")
	}
}

func TestPruneBackups_removeError(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"noted-2020-01-01.db", "noted-2020-01-02.db"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	chmodForTest(t, dir, 0o500) // readable, not writable

	if _, err := pruneBackups(dir, 1); err == nil {
		t.Fatal("pruneBackups on unwritable dir: want error")
	}
}

func TestBackup_pruneError_exits1(t *testing.T) {
	old := pruneBackupsFn
	pruneBackupsFn = func(string, int) (int, error) { return 0, fmt.Errorf("denied") }
	t.Cleanup(func() { pruneBackupsFn = old })

	_, errOut, code := runCLI(t, backupApp(t), []string{"backup", "--dir", backupDir(t)})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "prune") {
		t.Fatalf("stderr = %q, want prune error", errOut)
	}
}

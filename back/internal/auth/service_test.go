package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"noted/internal/database"
)

// This package deliberately runs tests sequentially: several of them swap
// process-wide knobs (bcryptCost, driver-level state) the same way
// cmd/server's shutdownTimeout tests do (TESTING.md exception).

// testEncKey builds a distinct valid 32-byte base64 TOTP_ENC_KEY per byte,
// so "wrong key" tests can pair two valid keys without tripping parseKey.
func testEncKey(b byte) string {
	k := make([]byte, 32)
	for i := range k {
		k[i] = b
	}
	return base64.StdEncoding.EncodeToString(k)
}

// newTestService opens a migrated temp-file database and a Service over it.
// bcryptCost drops to MinCost for the duration (see bcryptCost comment).
func newTestService(t *testing.T, encKey string) (*Service, *sql.DB) {
	t.Helper()
	return newTestServiceDir(t, t.TempDir(), encKey)
}

// newTestServiceDir is newTestService with an explicit directory, so tests
// that inspect the raw database bytes can find the file afterwards.
func newTestServiceDir(t *testing.T, dir, encKey string) (*Service, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	prev := bcryptCost
	bcryptCost = bcrypt.MinCost
	t.Cleanup(func() { bcryptCost = prev })
	return New(db, encKey), db
}

// enroll fully enrolls a fresh service and returns its secret + plaintext
// recovery codes, exactly as `server enroll` would.
func enroll(t *testing.T, s *Service) (secret string, codes []string) {
	t.Helper()
	key, err := s.Prep()
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	codes, err = s.CommitEnrollment(context.Background(), key.Secret())
	if err != nil {
		t.Fatalf("CommitEnrollment: %v", err)
	}
	return key.Secret(), codes
}

func TestEnrolled_reportsLifecycle(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))

	got, err := s.Enrolled(context.Background())
	if err != nil || got {
		t.Fatalf("before enroll: Enrolled() = %v, %v; want false, nil", got, err)
	}

	enroll(t, s)

	got, err = s.Enrolled(context.Background())
	if err != nil || !got {
		t.Fatalf("after enroll: Enrolled() = %v, %v; want true, nil", got, err)
	}
}

func TestEnrolled_closedDB_reportsError(t *testing.T) {
	s, db := newTestService(t, testEncKey(1))
	_ = db.Close()

	if _, err := s.Enrolled(context.Background()); err == nil {
		t.Fatal("Enrolled() on closed db: want error, got nil")
	}
}

func TestFormatTime_isUTCRFC3339(t *testing.T) {
	// Central European summer time must render as UTC "Z", matching every
	// auth column (API.md times are RFC3339 UTC).
	cest, err := time.Parse(time.RFC3339, "2026-10-04T14:00:00+02:00")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := formatTime(cest), "2026-10-04T12:00:00Z"; got != want {
		t.Fatalf("formatTime = %q, want %q", got, want)
	}
}

func TestRandomBytes_returnsRequestedLength(t *testing.T) {
	if got := len(randomBytes(32)); got != 32 {
		t.Fatalf("randomBytes(32) len = %d, want 32", got)
	}
}

func TestNew_storesEncKeyLazily(t *testing.T) {
	// An empty key must not panic or error at construction: boot without
	// TOTP_ENC_KEY is legal, only enroll/decrypt refuse.
	s, _ := newTestService(t, "")
	if s.encKey != "" {
		t.Fatalf("encKey = %q, want empty", s.encKey)
	}
	if err := s.CheckEncKey(); err == nil {
		t.Fatal("CheckEncKey with empty key: want error")
	}
}

package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"noted/internal/database"
)

var codeShape = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$`)

func TestCommitEnrollment_persistsSealedSecretAndHashedCodes(t *testing.T) {
	dir := t.TempDir()
	s, _ := newTestServiceDir(t, dir, testEncKey(1))

	key, err := s.Prep()
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	codes, err := s.CommitEnrollment(context.Background(), key.Secret())
	if err != nil {
		t.Fatalf("CommitEnrollment: %v", err)
	}
	if len(codes) != RECOVERY_CODE_COUNT {
		t.Fatalf("got %d codes, want %d", len(codes), RECOVERY_CODE_COUNT)
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if !codeShape.MatchString(c) {
			t.Fatalf("code %q does not match XXXX-XXXX Crockford shape", c)
		}
		if seen[c] {
			t.Fatalf("duplicate code %q", c)
		}
		seen[c] = true
	}

	// The secret must round-trip through the stored ciphertext...
	row, err := s.q.GetEnrollment(context.Background())
	if err != nil {
		t.Fatalf("GetEnrollment: %v", err)
	}
	plain, err := s.open(row.TotpSecret)
	if err != nil || plain != key.Secret() {
		t.Fatalf("stored secret opens to %q, %v; want %q", plain, err, key.Secret())
	}

	// ...and neither secret nor any code may appear in plaintext on disk
	// (WAL means the bytes can live in either file).
	var onDisk []byte
	for _, name := range []string{"auth.db", "auth.db-wal"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			onDisk = append(onDisk, b...)
		}
	}
	if strings.Contains(string(onDisk), key.Secret()) {
		t.Fatal("TOTP secret found in plaintext in the database file")
	}
	for _, c := range codes {
		if strings.Contains(string(onDisk), c) {
			t.Fatalf("recovery code %q found in plaintext in the database file", c)
		}
	}
}

func TestCommitEnrollment_secondEnrollment_conflicts(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	enroll(t, s)

	key, err := s.Prep()
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	_, err = s.CommitEnrollment(context.Background(), key.Secret())
	if !errors.Is(err, ErrEnrolled) {
		t.Fatalf("second CommitEnrollment = %v, want ErrEnrolled", err)
	}
}

func TestCommitEnrollment_failureBranches(t *testing.T) {
	t.Run("seal fails without key", func(t *testing.T) {
		s, _ := newTestService(t, "")
		if _, err := s.CommitEnrollment(context.Background(), "SECRET"); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("begin fails on canceled context", func(t *testing.T) {
		s, _ := newTestService(t, testEncKey(1))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := s.CommitEnrollment(ctx, "SECRET"); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("enrollment insert fails", func(t *testing.T) {
		// Dropping the table leaves BeginTx happy (no statement yet) and
		// fails the first insert with a non-UNIQUE error.
		s, db := newTestService(t, testEncKey(1))
		if _, err := db.Exec(`DROP TABLE enrollment`); err != nil {
			t.Fatal(err)
		}
		_, err := s.CommitEnrollment(context.Background(), "SECRET")
		if err == nil || errors.Is(err, ErrEnrolled) {
			t.Fatalf("want wrapped insert error, got %v", err)
		}
	})
	t.Run("recovery insert fails", func(t *testing.T) {
		// Enrollment inserts first; the pool table is gone, so the helper
		// fails and the whole transaction rolls back.
		s, db := newTestService(t, testEncKey(1))
		if _, err := db.Exec(`DROP TABLE recovery_codes`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CommitEnrollment(context.Background(), "SECRET"); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestRegenerateRecovery_rotatesTheWholePool(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	_, old := enroll(t, s)

	fresh, err := s.RegenerateRecovery(context.Background())
	if err != nil {
		t.Fatalf("RegenerateRecovery: %v", err)
	}
	if len(fresh) != RECOVERY_CODE_COUNT {
		t.Fatalf("got %d codes, want %d", len(fresh), RECOVERY_CODE_COUNT)
	}
	for _, c := range old {
		if _, err := s.Login(context.Background(), c); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("old code %q after rotation: %v, want ErrInvalidCode", c, err)
		}
	}
	if _, err := s.Login(context.Background(), fresh[0]); err != nil {
		t.Fatalf("fresh code after rotation: %v", err)
	}
}

func TestRegenerateRecovery_requiresEnrollment(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	if _, err := s.RegenerateRecovery(context.Background()); !errors.Is(err, ErrNotEnrolled) {
		t.Fatalf("RegenerateRecovery = %v, want ErrNotEnrolled", err)
	}
}

func TestRegenerateRecovery_failureBranches(t *testing.T) {
	t.Run("enrolled check fails on closed db", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		_ = db.Close()
		if _, err := s.RegenerateRecovery(context.Background()); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("begin fails on canceled context", func(t *testing.T) {
		s, _ := newTestService(t, testEncKey(1))
		enroll(t, s)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := s.RegenerateRecovery(ctx); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("enrollment lookup fails", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		enroll(t, s)
		if _, err := db.Exec(`DROP TABLE enrollment`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.RegenerateRecovery(context.Background()); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("burn fails", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		enroll(t, s)
		if _, err := db.Exec(`DROP TABLE recovery_codes`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.RegenerateRecovery(context.Background()); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("fresh insert fails", func(t *testing.T) {
		// The burn is an UPDATE; a BEFORE INSERT trigger fails only the
		// minting half, after the pool has already been marked used.
		s, db := newTestService(t, testEncKey(1))
		_, old := enroll(t, s)
		if _, err := db.Exec(`CREATE TRIGGER block_insert BEFORE INSERT ON recovery_codes
			BEGIN SELECT RAISE(ABORT, 'blocked'); END`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.RegenerateRecovery(context.Background()); err == nil {
			t.Fatal("want error")
		}
		// Rolled back as one unit: the burned pool must be restored.
		if _, err := s.Login(context.Background(), old[0]); err != nil {
			t.Fatalf("old code after failed rotation: %v", err)
		}
	})
}

func TestResetAuth_wipesEverything(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	secret, codes := enroll(t, s)
	token, err := s.Login(context.Background(), codes[0])
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := s.ResetAuth(context.Background()); err != nil {
		t.Fatalf("ResetAuth: %v", err)
	}
	if enrolled, err := s.Enrolled(context.Background()); err != nil || enrolled {
		t.Fatalf("Enrolled after reset = %v, %v; want false, nil", enrolled, err)
	}
	if _, err := s.Login(context.Background(), secret); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("recovery code after reset: %v, want ErrInvalidCode", err)
	}
	if _, err := s.Validate(context.Background(), token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("session after reset: %v, want ErrUnauthenticated", err)
	}
}

func TestResetAuth_failureBranches(t *testing.T) {
	t.Run("begin fails on canceled context", func(t *testing.T) {
		s, _ := newTestService(t, testEncKey(1))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := s.ResetAuth(ctx); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("clear enrollment fails", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		if _, err := db.Exec(`DROP TABLE enrollment`); err != nil {
			t.Fatal(err)
		}
		if err := s.ResetAuth(context.Background()); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("clear recovery codes fails", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		if _, err := db.Exec(`DROP TABLE recovery_codes`); err != nil {
			t.Fatal(err)
		}
		if err := s.ResetAuth(context.Background()); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("clear sessions fails", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		if _, err := db.Exec(`DROP TABLE sessions`); err != nil {
			t.Fatal(err)
		}
		if err := s.ResetAuth(context.Background()); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestNormalizeRecoveryCode_acceptsHumanTyping(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"ABCD-2345", "ABCD2345"},
		{"abcd2345", "ABCD2345"},
		{"abcd 2345", "ABCD2345"},
		{"ABCD 2345\t", "ABCD2345"},
		{"abcd-2345", "ABCD2345"},
		{"", ""},
		{"ABCD234", ""},    // too short
		{"ABCD-23456", ""}, // too long
		{"IIII-IIII", ""},  // I is not in the alphabet
		{"ABCD-234!", ""},  // symbol not in the alphabet
	} {
		if got := normalizeRecoveryCode(tc.in); got != tc.want {
			t.Fatalf("normalizeRecoveryCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGenerateRecoveryCodes_shapeAndCount(t *testing.T) {
	codes := generateRecoveryCodes(3)
	if len(codes) != 3 {
		t.Fatalf("got %d codes, want 3", len(codes))
	}
	for _, c := range codes {
		if !codeShape.MatchString(c) {
			t.Fatalf("code %q does not match XXXX-XXXX Crockford shape", c)
		}
	}
}

// TestMigrationsContainAuthTables keeps the raw-exec tests honest: they
// drop tables by name, so a rename must break here first.
func TestMigrationsContainAuthTables(t *testing.T) {
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, table := range []string{"enrollment", "recovery_codes", "sessions"} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

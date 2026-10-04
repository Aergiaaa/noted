package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestLogin_acceptsLiveTOTPAndMintsSession(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	secret, _ := enroll(t, s)

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	token, err := s.Login(context.Background(), code)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := s.Validate(context.Background(), token); err != nil {
		t.Fatalf("Validate(new token): %v", err)
	}
}

func TestLogin_emptyCode_isInvalid(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	if _, err := s.Login(context.Background(), ""); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("Login(\"\") = %v, want ErrInvalidCode", err)
	}
	if _, err := s.Login(context.Background(), "   "); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("Login(whitespace) = %v, want ErrInvalidCode", err)
	}
}

func TestLogin_wrongTOTP_isInvalid(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	secret, _ := enroll(t, s)

	// Five steps ahead is outside the ±1 acceptance window, so the miss is
	// deterministic rather than a 1-in-10^6 guess.
	code, err := totp.GenerateCode(secret, time.Now().Add(5*TOTP_STEP*time.Second))
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := s.Login(context.Background(), code); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("Login = %v, want ErrInvalidCode", err)
	}
}

func TestLogin_replayGuard_refusesSameCounter(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	secret, _ := enroll(t, s)
	base := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }

	code, err := totp.GenerateCode(secret, base)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := s.Login(context.Background(), code); err != nil {
		t.Fatalf("first Login: %v", err)
	}
	if _, err := s.Login(context.Background(), code); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("replayed Login = %v, want ErrInvalidCode", err)
	}
	// One step later the window has passed: same code, new counter window,
	// accepted again.
	s.now = func() time.Time { return base.Add(TOTP_STEP * time.Second) }
	code, err = totp.GenerateCode(secret, base.Add(TOTP_STEP*time.Second))
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := s.Login(context.Background(), code); err != nil {
		t.Fatalf("Login after window: %v", err)
	}
}

func TestAcceptCounter_windowRules(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	now := t0
	s.now = func() time.Time { return now }

	if !s.acceptCounter(7) {
		t.Fatal("first acceptance of a counter must succeed")
	}
	if s.acceptCounter(7) {
		t.Fatal("same counter within the window must be refused")
	}
	if !s.acceptCounter(8) {
		t.Fatal("a fresh counter must be accepted")
	}
	now = t0.Add(REPLAY_WINDOW)
	if !s.acceptCounter(8) {
		t.Fatal("same counter after the window must be accepted")
	}
}

func TestLogin_recoveryCode_acceptsHumanTyping(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	_, codes := enroll(t, s)

	lower := toLower(codes[0])
	token, err := s.Login(context.Background(), lower)
	if err != nil {
		t.Fatalf("Login with lowercase code: %v", err)
	}
	if _, err := s.Validate(context.Background(), token); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestLogin_recoveryCode_singleUse(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	_, codes := enroll(t, s)

	if _, err := s.Login(context.Background(), codes[0]); err != nil {
		t.Fatalf("first Login: %v", err)
	}
	if _, err := s.Login(context.Background(), codes[0]); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("second Login = %v, want ErrInvalidCode", err)
	}
}

func TestLogin_recoveryCode_unknownOrMalformed_isInvalid(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	enroll(t, s)

	for _, code := range []string{"ABCD-2345", "xx", "1234567890", "ABCD-23"} {
		if _, err := s.Login(context.Background(), code); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("Login(%q) = %v, want ErrInvalidCode", code, err)
		}
	}
}

func TestLogin_recoveryInternalFailures_areGenericInvalid(t *testing.T) {
	t.Run("list fails", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		_, codes := enroll(t, s)
		if _, err := db.Exec(`DROP TABLE recovery_codes`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Login(context.Background(), codes[0]); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("Login = %v, want ErrInvalidCode", err)
		}
	})
	t.Run("consume fails", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		_, codes := enroll(t, s)
		if _, err := db.Exec(`CREATE TRIGGER block_burn BEFORE UPDATE ON recovery_codes
			BEGIN SELECT RAISE(ABORT, 'blocked'); END`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Login(context.Background(), codes[0]); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("Login = %v, want ErrInvalidCode", err)
		}
	})
	t.Run("racing consumption loses", func(t *testing.T) {
		// RAISE(IGNORE) cancels the burn statement silently: the UPDATE
		// reports zero affected rows, exactly like a racing double-use.
		s, db := newTestService(t, testEncKey(1))
		_, codes := enroll(t, s)
		if _, err := db.Exec(`CREATE TRIGGER ignore_burn BEFORE UPDATE ON recovery_codes
			BEGIN SELECT RAISE(IGNORE); END`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Login(context.Background(), codes[0]); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("Login = %v, want ErrInvalidCode", err)
		}
	})
}

func TestLogin_totpVerificationError_isGenericInvalid(t *testing.T) {
	s, db := newTestService(t, testEncKey(1))
	enroll(t, s)
	_ = db.Close()
	if _, err := s.Login(context.Background(), "123456"); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("Login = %v, want ErrInvalidCode", err)
	}
}

func TestLogin_sessionWriteFailure_propagates(t *testing.T) {
	// Unlike credential failures, losing the session row is a real error:
	// the handler turns it into a 500 instead of a misleading 401.
	s, db := newTestService(t, testEncKey(1))
	secret, _ := enroll(t, s)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatal(err)
	}
	_, err = s.Login(context.Background(), code)
	if err == nil || errors.Is(err, ErrInvalidCode) {
		t.Fatalf("Login = %v, want a session-write error", err)
	}
}

// toLower lowercases a recovery code the way a human may type it.
func toLower(c string) string {
	b := []byte(c)
	for i, ch := range b {
		if ch >= 'A' && ch <= 'Z' {
			b[i] = ch + 32
		}
	}
	return string(b)
}

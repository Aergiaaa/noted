package auth

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestPrep_mintsUsableSecret(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))

	key, err := s.Prep()
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	if key.Secret() == "" {
		t.Fatal("Prep: empty secret")
	}
	if !strings.HasPrefix(key.URL(), "otpauth://totp/Noted:me") {
		t.Fatalf("Prep URI = %q, want otpauth://totp/Noted:me prefix", key.URL())
	}
}

func TestPrep_missingKey_failsBeforePrompt(t *testing.T) {
	s, _ := newTestService(t, "")
	if _, err := s.Prep(); err == nil {
		t.Fatal("Prep with no TOTP_ENC_KEY: want error")
	}
}

func TestCheckEncKey_rejectsMalformedKeys(t *testing.T) {
	for _, tc := range []struct {
		name, key string
	}{
		{"empty", ""},
		{"not base64", "!!not-base64!!"},
		{"short key", base64.StdEncoding.EncodeToString(make([]byte, 16))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newTestService(t, tc.key)
			if err := s.CheckEncKey(); err == nil {
				t.Fatalf("CheckEncKey(%q): want error", tc.key)
			}
		})
	}
}

func TestSealOpen_roundTrip(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))

	sealed, err := s.seal("SUPERSECRET")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if strings.Contains(sealed, "SUPERSECRET") {
		t.Fatal("sealed value contains plaintext")
	}
	got, err := s.open(sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got != "SUPERSECRET" {
		t.Fatalf("open = %q, want SUPERSECRET", got)
	}
}

func TestSeal_missingKey_fails(t *testing.T) {
	s, _ := newTestService(t, "")
	if _, err := s.seal("x"); err == nil {
		t.Fatal("seal with no key: want error")
	}
}

func TestOpen_rejectsCorruptBlobs(t *testing.T) {
	t.Run("missing key", func(t *testing.T) {
		s, _ := newTestService(t, "")
		if _, err := s.open("AAAA"); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("bad base64", func(t *testing.T) {
		s, _ := newTestService(t, testEncKey(1))
		if _, err := s.open("!!nope!!"); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("truncated", func(t *testing.T) {
		s, _ := newTestService(t, testEncKey(1))
		// GCM nonces are 12 bytes; anything shorter cannot even split.
		short := base64.StdEncoding.EncodeToString([]byte("tiny"))
		if _, err := s.open(short); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("wrong key", func(t *testing.T) {
		// Enroll under key A, decrypt under valid key B: GCM auth fails.
		a, _ := newTestService(t, testEncKey(1))
		sealed, err := a.seal("SECRET")
		if err != nil {
			t.Fatalf("seal: %v", err)
		}
		b, _ := newTestService(t, testEncKey(2))
		if _, err := b.open(sealed); err == nil {
			t.Fatal("open under wrong key: want error")
		}
	})
	t.Run("tampered ciphertext", func(t *testing.T) {
		s, _ := newTestService(t, testEncKey(1))
		sealed, err := s.seal("SECRET")
		if err != nil {
			t.Fatalf("seal: %v", err)
		}
		raw, err := base64.StdEncoding.DecodeString(sealed)
		if err != nil {
			t.Fatal(err)
		}
		raw[len(raw)-1] ^= 0xFF
		if _, err := s.open(base64.StdEncoding.EncodeToString(raw)); err == nil {
			t.Fatal("open tampered: want error")
		}
	})
}

func TestValidateCode_acceptsAdjacentSteps(t *testing.T) {
	// Freeze time so the ±1-step window is exact: a code minted one step
	// before or after "now" must match, two steps away must not.
	s, _ := newTestService(t, testEncKey(1))
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	secret, _ := enroll(t, s)

	for _, tc := range []struct {
		name       string
		codeAt     time.Time
		wantOK     bool
		wantCounts bool
	}{
		{"current step", now, true, true},
		{"previous step", now.Add(-time.Duration(TOTP_STEP) * time.Second), true, true},
		{"next step", now.Add(time.Duration(TOTP_STEP) * time.Second), true, true},
		{"two steps ago", now.Add(-2 * time.Duration(TOTP_STEP) * time.Second), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, err := totp.GenerateCode(secret, tc.codeAt)
			if err != nil {
				t.Fatalf("GenerateCode: %v", err)
			}
			ok, counter, err := s.ValidateCode(context.Background(), code)
			if err != nil {
				t.Fatalf("ValidateCode: %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (counter %d)", ok, tc.wantOK, counter)
			}
			if ok && counter != uint64(tc.codeAt.Unix()/int64(TOTP_STEP)) {
				t.Fatalf("counter = %d, want %d", counter, tc.codeAt.Unix()/int64(TOTP_STEP))
			}
		})
	}
}

func TestValidateCode_withoutEnrollment_isPlainInvalid(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	ok, _, err := s.ValidateCode(context.Background(), "123456")
	if ok || err != nil {
		t.Fatalf("ValidateCode = %v, %v; want false, nil", ok, err)
	}
}

func TestValidateCode_internalFailures_reportError(t *testing.T) {
	t.Run("closed db", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		_ = db.Close()
		if _, _, err := s.ValidateCode(context.Background(), "123456"); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("undecryptable secret", func(t *testing.T) {
		s, db := newTestService(t, testEncKey(1))
		enroll(t, s)
		other := New(db, testEncKey(2)) // same DB, valid but wrong key
		if _, _, err := other.ValidateCode(context.Background(), "123456"); err == nil {
			t.Fatal("want error")
		}
	})
}

// TestValidateCandidate_checksInMemorySecret pins the enrollment path:
// nothing is in the DB yet, so the ±1-step window must run against the
// secret that is about to be committed.
func TestValidateCandidate_checksInMemorySecret(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	key, err := s.Prep()
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}

	for _, tc := range []struct {
		name   string
		codeAt time.Time
		secret string // secret the candidate is checked against
		want   bool
	}{
		{"current step", now, key.Secret(), true},
		{"previous step", now.Add(-time.Duration(TOTP_STEP) * time.Second), key.Secret(), true},
		{"next step", now.Add(time.Duration(TOTP_STEP) * time.Second), key.Secret(), true},
		{"two steps away", now.Add(-2 * time.Duration(TOTP_STEP) * time.Second), key.Secret(), false},
		{"wrong secret", now, "JBSWY3DPEHPK3PXP", false},
		{"not a code", now, key.Secret(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code := "000000"
			if tc.name != "not a code" {
				// Always mint from the in-memory enrollment secret; the
				// candidate is then checked against tc.secret.
				var gerr error
				code, gerr = totp.GenerateCode(key.Secret(), tc.codeAt)
				if gerr != nil {
					t.Fatalf("GenerateCode: %v", gerr)
				}
			}
			ok, _ := s.ValidateCandidate(code, tc.secret)
			if ok != tc.want {
				t.Fatalf("ok = %v, want %v", ok, tc.want)
			}
		})
	}
}

func TestIsTOTPCode_separatesLoginShapes(t *testing.T) {
	for _, tc := range []struct {
		code string
		want bool
	}{
		{"123456", true},
		{"000000", true},
		{"12345", false},    // 5 digits
		{"1234567", false},  // 7 digits
		{"12345a", false},   // digit + letter
		{"ABCD2345", false}, // recovery shape (8 symbols)
		{"", false},
	} {
		if got := isTOTPCode(tc.code); got != tc.want {
			t.Fatalf("isTOTPCode(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

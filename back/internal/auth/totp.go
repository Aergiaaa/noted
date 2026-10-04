package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/hotp"
	"github.com/pquerna/otp/totp"
)

const (
	// ISSUER/ACCOUNT label the otpauth URI that `server enroll` prints.
	// Single-user app: one account, the host owner.
	ISSUER  = "Noted"
	ACCOUNT = "me"

	// TOTP_STEP is the RFC6238 period in seconds (pquerna/otp's default
	// of 30, stated here so the replay math below does not depend on it).
	TOTP_STEP = 30
)

// Prep mints a fresh TOTP secret + otpauth URI for enrollment. Nothing is
// persisted here: CommitEnrollment does that only after the operator proves
// a live code (SECURITY.md — host access is the enrollment credential).
func (s *Service) Prep() (*otp.Key, error) {
	if err := s.CheckEncKey(); err != nil {
		return nil, err
	}
	return totp.Generate(totp.GenerateOpts{Issuer: ISSUER, AccountName: ACCOUNT})
}

// CheckEncKey parses TOTP_ENC_KEY up front so the CLI can fail before it
// prompts for a code, not after.
func (s *Service) CheckEncKey() error {
	_, err := s.parseKey()
	return err
}

// parseKey base64-decodes TOTP_ENC_KEY into the 32-byte AES key the secret
// is sealed with.
func (s *Service) parseKey() ([]byte, error) {
	if s.encKey == "" {
		return nil, fmt.Errorf("TOTP_ENC_KEY is not set")
	}
	raw, err := base64.StdEncoding.DecodeString(s.encKey)
	if err != nil {
		return nil, fmt.Errorf("TOTP_ENC_KEY: decode: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("TOTP_ENC_KEY: got %d bytes, want 32", len(raw))
	}
	return raw, nil
}

// gcm builds the AEAD. parseKey already guarantees a 32-byte key, which is
// a valid AES-256 key and has a fixed 16-byte block, so the two constructors
// below cannot fail; their errors are dropped instead of branched on (an
// unreachable branch would fail the coverage gate for no safety gain).
func (s *Service) gcm() (cipher.AEAD, error) {
	key, err := s.parseKey()
	if err != nil {
		return nil, err
	}
	block, _ := aes.NewCipher(key) // valid: parseKey enforces 32 bytes
	gcm, _ := cipher.NewGCM(block) // valid: AES block size is always 16
	return gcm, nil
}

// seal encrypts the TOTP secret at rest: base64(nonce || AES-GCM(ct)),
// 12-byte random nonce (SECURITY.md).
func (s *Service) seal(plain string) (string, error) {
	g, err := s.gcm()
	if err != nil {
		return "", err
	}
	nonce := randomBytes(g.NonceSize())
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(plain), nil)), nil
}

// open reverses seal. Any failure (bad base64, truncated blob, wrong key,
// tampered ciphertext) is an error — callers surface it as an auth failure
// with the detail only in the server log.
func (s *Service) open(sealed string) (string, error) {
	g, err := s.gcm()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", fmt.Errorf("secret decode: %w", err)
	}
	if len(raw) < g.NonceSize() {
		return "", fmt.Errorf("secret truncated")
	}
	plain, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("secret decrypt: %w", err)
	}
	return string(plain), nil
}

// ValidateCode reports whether code matches the enrolled secret within
// ±1 30s step, and which HOTP counter matched (the replay guard's input).
// No enrollment row means "no match", not an error: a login attempt learns
// nothing about whether the host has enrolled.
func (s *Service) ValidateCode(ctx context.Context, code string) (ok bool, counter uint64, err error) {
	row, err := s.q.GetEnrollment(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("get enrollment: %w", err)
	}
	secret, err := s.open(row.TotpSecret)
	if err != nil {
		return false, 0, fmt.Errorf("open secret: %w", err)
	}
	ok, counter = s.validateWithSecret(code, secret)
	return ok, counter, nil
}

// ValidateCandidate checks a live code against an in-memory secret — the
// enrollment path, where nothing is stored yet, so the window check must
// run against the secret being enrolled rather than the database row.
func (s *Service) ValidateCandidate(code, secret string) (ok bool, counter uint64) {
	return s.validateWithSecret(code, secret)
}

// validateWithSecret is the shared ±1-step window check: counters base-1,
// base and base+1 of the service clock. base-1 underflows modulo 2^64, so
// the wrap is deliberate arithmetic.
func (s *Service) validateWithSecret(code, secret string) (ok bool, counter uint64) {
	base := uint64(s.now().Unix() / TOTP_STEP)
	for _, d := range []int64{-1, 0, 1} {
		if hotp.Validate(code, base+uint64(d), secret) {
			return true, base + uint64(d)
		}
	}
	return false, 0
}

// isTOTPCode separates the two login shapes: TOTP is exactly 6 digits,
// recovery codes are always RECOVERY_CODE_LEN symbols of a 32-char
// alphabet, so the domains never overlap.
func isTOTPCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

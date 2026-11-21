package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/hotp"
	"github.com/pquerna/otp/totp"
)

const (
	ISSUER  = "Noted"
	ACCOUNT = "me"

	// TOTP_STEP is the RFC6238 period, stated so the window math below
	// does not depend on pquerna/otp's default.
	TOTP_STEP = 30
)

func (s *Service) Prep() (*otp.Key, error) {
	if err := s.CheckEncKey(); err != nil {
		return nil, err
	}

	return totp.Generate(totp.GenerateOpts{
		Issuer:      ISSUER,
		AccountName: ACCOUNT,
	})
}

// SECRET_BYTES matches totp.Generate's 20-byte secret (RFC4226).
const SECRET_BYTES = 20

// NormalizeSecret validates an operator-supplied base32 secret (the CLI's
// --secret flag, used to restore an authenticator backup or seed an
// automated test that already owns the secret). Uppercase and padding-free,
// exactly the shape totp.Generate mints; anything else is a usage error.
func NormalizeSecret(secret string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(secret))
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil {
		return "", fmt.Errorf("secret: %w", err)
	}

	if len(raw) != SECRET_BYTES {
		return "", fmt.Errorf("secret: got %d bytes, want %d", len(raw), SECRET_BYTES)
	}

	return normalized, nil
}

// EnrollURI renders the otpauth:// URI for a supplied secret. SHA1/6
// digits/30s are the defaults an authenticator assumes, matching
// totp.Generate's key.
func EnrollURI(secret string) string {
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", ISSUER)
	return fmt.Sprintf("otpauth://totp/%s:%s?%s",
		url.PathEscape(ISSUER), url.PathEscape(ACCOUNT), q.Encode())
}

// CheckEncKey parses TOTP_ENC_KEY up front so the CLI can fail before it
// prompts for a code, not after.
func (s *Service) CheckEncKey() error {
	_, err := s.parseKey()
	return err
}

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

// gcm: parseKey guarantees a valid AES-256 key, so both constructors
// cannot fail — their errors are dropped, not branched on (an unreachable
// branch would fail the coverage gate for no safety gain).
func (s *Service) gcm() (cipher.AEAD, error) {
	key, err := s.parseKey()
	if err != nil {
		return nil, err
	}

	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)

	return gcm, nil
}

func (s *Service) seal(plain string) (string, error) {
	g, err := s.gcm()
	if err != nil {
		return "", err
	}

	nonce := randomBytes(g.NonceSize())

	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(plain), nil)), nil
}

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

// ValidateCode: a missing enrollment row means "no match", not an error —
// a login attempt learns nothing about whether the host has enrolled.
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

func (s *Service) ValidateCandidate(code, secret string) (ok bool, counter uint64) {
	return s.validateWithSecret(code, secret)
}

// validateWithSecret checks counters base-1, base and base+1 of the service
// clock. base-1 underflows modulo 2^64, so the wrap is deliberate arithmetic.
func (s *Service) validateWithSecret(code, secret string) (ok bool, counter uint64) {
	base := uint64(s.now().Unix() / TOTP_STEP)

	for _, d := range []int64{-1, 0, 1} {
		if hotp.Validate(code, base+uint64(d), secret) {
			return true, base + uint64(d)
		}
	}

	return false, 0
}

// isTOTPCode separates the login shapes: exactly 6 digits versus an
// 8-symbol recovery code — the two domains never overlap.
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

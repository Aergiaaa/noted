package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"noted/internal/database/generated"
)

// CROCKFORD is Crockford base32 without I/L/O/U (unambiguous by eye):
// 32 symbols = exactly 5 bits each, so codes are drawn from whole bytes
// with zero modulo bias.
const CROCKFORD = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// bcryptCost is a var so tests can drop to bcrypt.MinCost — ten cost-10
// hashes per enrollment would otherwise slow every enrolling test for no
// gain against a 40-bit code in a temp DB. Tests that swap it must not run
// in parallel with the rest of the package.
var bcryptCost = bcrypt.DefaultCost

// CommitEnrollment persists the sealed secret plus a fresh recovery pool in
// one transaction and returns the plaintext codes — printed once by the
// CLI, never stored.
func (s *Service) CommitEnrollment(ctx context.Context, secret string) ([]string, error) {
	sealed, err := s.seal(secret)
	if err != nil {
		return nil, err
	}

	now := formatTime(s.now())
	codes := generateRecoveryCodes(RECOVERY_CODE_COUNT)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)
	n, err := q.CreateEnrollment(ctx, generated.CreateEnrollmentParams{
		TotpSecret: sealed,
		EnrolledAt: &now,
		UpdatedAt:  now,
	})
	if err != nil {
		return nil, fmt.Errorf("insert enrollment: %w", err)
	}

	if n == 0 {
		return nil, ErrEnrolled // ON CONFLICT (id) DO NOTHING: row exists
	}

	if err := insertRecoveryCodes(ctx, q, codes, now); err != nil {
		return nil, err
	}

	return codes, tx.Commit()
}

// RegenerateRecovery burns every unused code first, so a leak dies with its
// siblings, then mints fresh ones in the same transaction — the pool can
// never be empty nor doubled.
func (s *Service) RegenerateRecovery(ctx context.Context) ([]string, error) {
	now := formatTime(s.now())
	codes := generateRecoveryCodes(RECOVERY_CODE_COUNT)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)
	if _, err := q.GetEnrollment(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotEnrolled
		}

		return nil, fmt.Errorf("get enrollment: %w", err)
	}

	if err := q.MarkAllRecoveryCodesUsed(ctx, &now); err != nil {
		return nil, fmt.Errorf("burn recovery codes: %w", err)
	}

	if err := insertRecoveryCodes(ctx, q, codes, now); err != nil {
		return nil, err
	}

	return codes, tx.Commit()
}

// RotateEncKey re-encrypts the stored TOTP secret with a new random key.
// The current key (from TOTP_ENC_KEY at service construction) decrypts the
// existing sealed secret; the new key seals it. The service's encKey is
// updated so the running instance can decrypt with the new key too. The
// operator must update TOTP_ENC_KEY in the environment and restart for
// the change to persist across restarts — loss of the key means loss of
// the enrollment (SECURITY.md). Returns the new base64 key for the
// operator to record.
func (s *Service) RotateEncKey(ctx context.Context) (string, error) {
	// Generate the new key: 32 random bytes, base64-encoded.
	newKeyBytes := randomBytes(32)
	newKey := base64.StdEncoding.EncodeToString(newKeyBytes)

	row, err := s.q.GetEnrollment(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotEnrolled
	}
	if err != nil {
		return "", fmt.Errorf("get enrollment: %w", err)
	}

	// Decrypt with the current key.
	secret, err := s.open(row.TotpSecret)
	if err != nil {
		return "", fmt.Errorf("decrypt with current key: %w", err)
	}

	// Swap to the new key, seal, and persist. On failure, restore the old
	// key so the service can still decrypt with it. seal cannot fail here:
	// newKey is base64 of 32 random bytes, so parseKey/aes.NewCipher/
	// NewGCM all succeed.
	oldKey := s.encKey
	s.encKey = newKey
	sealed, _ := s.seal(secret)

	now := formatTime(s.now())
	if err := s.q.UpdateEnrollment(ctx, generated.UpdateEnrollmentParams{
		TotpSecret: sealed,
		UpdatedAt:  now,
	}); err != nil {
		s.encKey = oldKey
		return "", fmt.Errorf("update enrollment: %w", err)
	}

	return newKey, nil
}

// RotateRecovery gates the pool rotation on a TOTP-minted session: a
// stolen session must not be able to mint fresh recovery codes, which
// would outlive any logout (SECURITY.md). Expiry itself was already
// checked by RequireSession in front of the handler.
func (s *Service) RotateRecovery(ctx context.Context, token string) ([]string, error) {
	row, err := s.q.GetSessionByTokenHash(ctx, hashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthenticated
	}

	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	if row.AuthMethod != AUTH_METHOD_TOTP {
		return nil, ErrStepUpRequired
	}

	return s.RegenerateRecovery(ctx)
}

func (s *Service) ResetAuth(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)
	if err := q.ClearEnrollment(ctx); err != nil {
		return fmt.Errorf("clear enrollment: %w", err)
	}

	if err := q.DeleteAllRecoveryCodes(ctx); err != nil {
		return fmt.Errorf("clear recovery codes: %w", err)
	}

	if err := q.DeleteAllSessions(ctx); err != nil {
		return fmt.Errorf("clear sessions: %w", err)
	}

	return tx.Commit()
}

// insertRecoveryCodes hashes one code per row. bcrypt's error is dropped
// deliberately: its only failure is an input past 72 bytes, and the
// normalization guard below proves the input is exactly RECOVERY_CODE_LEN
// symbols first. Codes never appear in errors or logs.
func insertRecoveryCodes(ctx context.Context, q *generated.Queries, codes []string, at string) error {
	for _, c := range codes {
		normalized := normalizeRecoveryCode(c)
		if normalized == "" {
			return errors.New("recovery code failed normalization")
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(normalized), bcryptCost)

		err := q.CreateRecoveryCode(ctx, generated.CreateRecoveryCodeParams{
			ID:        uuid.NewString(),
			CodeHash:  string(hash),
			CreatedAt: at,
		})
		if err != nil {
			return fmt.Errorf("insert recovery code: %w", err)
		}
	}

	return nil
}

func generateRecoveryCodes(count int) []string {
	codes := make([]string, 0, count)
	for len(codes) < count {
		codes = append(codes, newRecoveryCode())
	}

	return codes
}

func newRecoveryCode() string {
	var v uint64
	for _, b := range randomBytes(RECOVERY_CODE_LEN * 5 / 8) {
		v = v<<8 | uint64(b)
	}

	chars := make([]byte, RECOVERY_CODE_LEN)
	for i := range chars {
		chars[i] = CROCKFORD[(v>>uint(35-5*i))&31]
	}

	return string(chars[:4]) + "-" + string(chars[4:])
}

func normalizeRecoveryCode(code string) string {
	clean := strings.Map(recoveryRune, code)

	if len(clean) != RECOVERY_CODE_LEN {
		return ""
	}

	for _, r := range clean {
		if !strings.ContainsRune(CROCKFORD, r) {
			return ""
		}
	}

	return clean
}

func recoveryRune(r rune) rune {
	switch {
	case r == '-' || r == ' ' || r == '\t':
		return -1
	case r >= 'a' && r <= 'z':
		r = r - 'a' + 'A'
	}

	// Crockford folds I/L to 1 and O to 0 so a transcription typo is
	// corrected, not rejected; generated codes never contain them, so a
	// fold can only ever rescue a human.
	switch r {
	case 'I', 'L':
		return '1'
	case 'O':
		return '0'
	default:
		return r
	}
}

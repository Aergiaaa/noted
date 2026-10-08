package auth

import (
	"context"
	"database/sql"
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
	err = q.CreateEnrollment(ctx, generated.CreateEnrollmentParams{
		TotpSecret: sealed,
		EnrolledAt: &now,
		UpdatedAt:  now,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrEnrolled
		}

		return nil, fmt.Errorf("insert enrollment: %w", err)
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

// bcrypt's error is dropped: cost is a package constant and codes are 8
// bytes, far under the 72-byte limit, so GenerateFromPassword cannot fail.
func insertRecoveryCodes(ctx context.Context, q *generated.Queries, codes []string, at string) error {
	for _, c := range codes {
		hash, _ := bcrypt.GenerateFromPassword([]byte(normalizeRecoveryCode(c)), bcryptCost)
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
		return r - 'a' + 'A'
	default:
		return r
	}
}

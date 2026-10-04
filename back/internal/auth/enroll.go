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
// 32 symbols = exactly 5 bits, so RECOVERY_CODE_LEN chars are drawn from
// whole bytes (40 bits of 5 random bytes) with zero modulo bias.
const CROCKFORD = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// bcryptCost is a var — same trick as cmd/server's shutdownTimeout — so
// tests can drop to bcrypt.MinCost: ten cost-10 hashes per enrollment would
// otherwise add half a second to every test that enrolls, for no security
// gain against a 40-bit code in a temp database. Tests that swap it must
// not run in parallel with the rest of the package.
var bcryptCost = bcrypt.DefaultCost

// CommitEnrollment persists a sealed secret plus a fresh recovery pool in
// one BEGIN IMMEDIATE transaction and returns the plaintext codes — printed
// once by the CLI, never stored plaintext. The secret must come from Prep.
// A second enrollment loses on the id=1 primary key (ErrEnrolled).
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
	q := s.q.WithTx(tx)
	err = q.CreateEnrollment(ctx, generated.CreateEnrollmentParams{
		TotpSecret: sealed,
		EnrolledAt: &now,
		UpdatedAt:  now,
	})
	if err != nil {
		_ = tx.Rollback()
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrEnrolled
		}
		return nil, fmt.Errorf("insert enrollment: %w", err)
	}
	if err := insertRecoveryCodes(ctx, q, codes, now); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return codes, tx.Commit()
}

// RegenerateRecovery rotates the pool (POST /api/auth/recovery-codes):
// every unused code is burned first so a leak dies with its siblings, then
// fresh ones are minted — same transaction, so the pool can never be empty
// nor doubled. Returns the new plaintext codes once. No enrollment means
// ErrNotEnrolled (the handler's guard makes that unreachable over HTTP).
func (s *Service) RegenerateRecovery(ctx context.Context) ([]string, error) {
	now := formatTime(s.now())
	codes := generateRecoveryCodes(RECOVERY_CODE_COUNT)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	q := s.q.WithTx(tx)
	if _, err := q.GetEnrollment(ctx); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotEnrolled
		}
		return nil, fmt.Errorf("get enrollment: %w", err)
	}
	if err := q.MarkAllRecoveryCodesUsed(ctx, &now); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("burn recovery codes: %w", err)
	}
	if err := insertRecoveryCodes(ctx, q, codes, now); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return codes, tx.Commit()
}

// ResetAuth is `server reset-auth --yes`: enrollment, recovery pool and
// every session are wiped so the host can re-enroll from scratch.
func (s *Service) ResetAuth(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	q := s.q.WithTx(tx)
	if err := q.ClearEnrollment(ctx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear enrollment: %w", err)
	}
	if err := q.DeleteAllRecoveryCodes(ctx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear recovery codes: %w", err)
	}
	if err := q.DeleteAllSessions(ctx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear sessions: %w", err)
	}
	return tx.Commit()
}

// insertRecoveryCodes persists plaintext codes as bcrypt hashes of their
// normalized form (that is what Login compares against — typing aid: hyphen
// or not, case or not, same code). bcrypt's error is dropped: cost is a
// package constant and codes are 8 bytes, far under the 72-byte limit, so
// GenerateFromPassword cannot fail here. Hashes are salted, so the UNIQUE
// index never trips on a repeat.
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

// generateRecoveryCodes builds count fresh XXXX-XXXX codes.
func generateRecoveryCodes(count int) []string {
	codes := make([]string, 0, count)
	for len(codes) < count {
		codes = append(codes, newRecoveryCode())
	}
	return codes
}

// newRecoveryCode draws 40 random bits and splits them into 8 5-bit
// symbols, grouped XXXX-XXXX for readability (SECURITY.md).
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

// normalizeRecoveryCode accepts what a human may type — lowercase, spaces,
// the separating hyphen — and returns the canonical 8-symbol form, or ""
// when the input cannot be a recovery code at all.
func normalizeRecoveryCode(code string) string {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r == '-' || r == ' ' || r == '\t':
			return -1
		case r >= 'a' && r <= 'z':
			return r - 'a' + 'A'
		default:
			return r
		}
	}, code)
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

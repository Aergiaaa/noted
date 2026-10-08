// Package auth never logs codes, secrets or tokens.
package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"noted/internal/database/generated"
)

const (
	SESSION_COOKIE     = "noted_session"
	COOKIE_MAX_AGE_SEC = 2592000

	SESSION_TTL          = 30 * 24 * time.Hour
	SESSION_REFRESH_AT   = 7 * 24 * time.Hour
	SESSION_ABSOLUTE_MAX = 90 * 24 * time.Hour

	RECOVERY_CODE_COUNT = 10
	RECOVERY_CODE_LEN   = 8

	REPLAY_WINDOW = 30 * time.Second

	LOGIN_RATE_LIMIT  = 5
	LOGIN_RATE_WINDOW = time.Minute
)

// ErrInvalidCode is the only failure a login attempt may reveal: one
// generic 401 whether the code is wrong, spent, or unenrolled — no
// probing, no enumeration (SECURITY.md).
var (
	ErrInvalidCode     = errors.New("invalid code")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrEnrolled        = errors.New("already enrolled")
	ErrNotEnrolled     = errors.New("not enrolled")
)

// Service keeps db alongside the queries because multi-statement writes
// open a BEGIN IMMEDIATE transaction (DATABASE.md); now is a field so
// tests can pin time.
type Service struct {
	db     *sql.DB
	q      *generated.Queries
	encKey string
	now    func() time.Time

	mu          sync.Mutex
	lastCounter uint64
	lastCodeAt  time.Time
}

// New stores encKey verbatim and parses it lazily, so a server without
// TOTP_ENC_KEY still boots — it just cannot enroll or decrypt.
func New(db *sql.DB, encKey string) *Service {
	return &Service{
		db:     db,
		q:      generated.New(db),
		encKey: encKey,
		now:    time.Now,
	}
}

func (s *Service) Enrolled(ctx context.Context) (bool, error) {
	_, err := s.q.GetEnrollment(ctx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("get enrollment: %w", err)
	}

	return true, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// randomBytes discards crypto/rand.Read's error deliberately: the
// documented contract is that it never fails, so the branch would be
// unreachable — and fail the coverage gate for no safety gain.
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

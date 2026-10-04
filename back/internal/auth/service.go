// Package auth owns every authentication state transition: host-local
// enrollment (`server enroll`), TOTP/recovery-code login, session minting
// and validation, and recovery-code rotation (ARCHITECTURE.md service
// layer). It never logs codes, secrets or tokens, and carries no HTTP
// beyond the session cookie's name.
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
	// SESSION_COOKIE / COOKIE_MAX_AGE: cookie identity and Max-Age in
	// seconds (30d), per SECURITY.md.
	SESSION_COOKIE     = "noted_session"
	COOKIE_MAX_AGE_SEC = 2592000

	// SESSION_TTL slides forward on use; SESSION_REFRESH_AT decides when
	// (re-issue if less than this remains) and SESSION_ABSOLUTE_MAX caps
	// every re-issue at created_at + 90d (SECURITY.md).
	SESSION_TTL          = 30 * 24 * time.Hour
	SESSION_REFRESH_AT   = 7 * 24 * time.Hour
	SESSION_ABSOLUTE_MAX = 90 * 24 * time.Hour

	// RECOVERY_CODE_COUNT codes are minted per enrollment / rotation
	// (SECURITY.md), each RECOVERY_CODE_LEN chars of a 5-bit alphabet.
	RECOVERY_CODE_COUNT = 10
	RECOVERY_CODE_LEN   = 8

	// REPLAY_WINDOW: the same accepted TOTP counter is refused again
	// within this window (in-memory, reset on restart — SECURITY.md).
	REPLAY_WINDOW = 30 * time.Second

	// LOGIN_RATE_LIMIT / LOGIN_RATE_WINDOW: per-IP token bucket on
	// POST /api/auth/login — every attempt counts, the 6th inside the
	// window gets 429 + Retry-After (SECURITY.md).
	LOGIN_RATE_LIMIT  = 5
	LOGIN_RATE_WINDOW = time.Minute
)

// Errors the HTTP/CLI layers map to their frozen responses. ErrInvalidCode
// is deliberately the only failure a login attempt reveals (API.md: generic
// 401, no probing); internal failures wrap nothing user-facing.
var (
	ErrInvalidCode     = errors.New("invalid code")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrEnrolled        = errors.New("already enrolled")
	ErrNotEnrolled     = errors.New("not enrolled")
)

// Service is the auth state machine over sqlc queries. db is kept alongside
// the queries because multi-statement writes open a BEGIN IMMEDIATE
// transaction (DATABASE.md); now is a field so tests can pin time.
type Service struct {
	db     *sql.DB
	q      *generated.Queries
	encKey string // base64 32B AES key, parsed on use by parseKey
	now    func() time.Time

	mu          sync.Mutex
	lastCounter uint64    // replay guard: counter of the last accepted TOTP
	lastCodeAt  time.Time // when that counter was accepted
}

// New builds a Service over an open (migrated) database. encKey is stored
// verbatim and parsed lazily: a server without TOTP_ENC_KEY still boots
// (it just cannot enroll or decrypt), which keeps `make dev` working.
func New(db *sql.DB, encKey string) *Service {
	return &Service{db: db, q: generated.New(db), encKey: encKey, now: time.Now}
}

// Enrolled reports whether `server enroll` has committed a secret.
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

// formatTime renders an instant the way every auth column is stored:
// UTC RFC3339 (API.md conventions).
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// randomBytes fills n bytes from the OS CSPRNG. crypto/rand.Read is
// documented never to fail (Go 1.24+), so the error is discarded rather
// than branched on: the resulting branch would be unreachable in tests,
// and the contract is stronger than any handling we could add here.
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

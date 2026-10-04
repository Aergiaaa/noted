package auth

import (
	"context"
	"log"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"noted/internal/database/generated"
)

// Login authenticates a 6-digit TOTP (±1 step) or an unused recovery code
// and mints a session token. Every credential failure is ErrInvalidCode —
// one generic answer whether the host has enrolled, the code is wrong, or
// it was already spent (SECURITY.md: no probing, no enumeration). Internal
// failures (DB, decryption) are logged and reported the same way, except a
// session write failure, which propagates as a real error.
func (s *Service) Login(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ErrInvalidCode
	}
	if isTOTPCode(code) {
		return s.loginTOTP(ctx, code)
	}
	return s.loginRecovery(ctx, code)
}

// loginTOTP verifies the code against the enrolled secret, then runs the
// replay guard before minting the session.
func (s *Service) loginTOTP(ctx context.Context, code string) (string, error) {
	ok, counter, err := s.ValidateCode(ctx, code)
	if err != nil {
		log.Printf("auth: totp verify: %v", err)
		return "", ErrInvalidCode
	}
	if !ok {
		return "", ErrInvalidCode
	}
	if !s.acceptCounter(counter) {
		return "", ErrInvalidCode
	}
	return s.createSession(ctx)
}

// acceptCounter is the replay guard: the same counter is accepted once per
// REPLAY_WINDOW. In-memory on purpose — a restart forgets it, a trade-off
// documented in SECURITY.md.
func (s *Service) acceptCounter(counter uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if counter == s.lastCounter && now.Sub(s.lastCodeAt) < REPLAY_WINDOW {
		return false
	}
	s.lastCounter, s.lastCodeAt = counter, now
	return true
}

// loginRecovery scans the unused pool (bcrypt hashes are salted, so a
// direct lookup is impossible) and consumes the match before minting the
// session. Consumption is a conditional UPDATE: losing a race still yields
// the generic invalid-code answer, never a second use of one code.
func (s *Service) loginRecovery(ctx context.Context, code string) (string, error) {
	want := normalizeRecoveryCode(code)
	if want == "" {
		return "", ErrInvalidCode
	}
	rows, err := s.q.ListUnusedRecoveryCodes(ctx)
	if err != nil {
		log.Printf("auth: recovery list: %v", err)
		return "", ErrInvalidCode
	}
	for _, r := range rows {
		if bcrypt.CompareHashAndPassword([]byte(r.CodeHash), []byte(want)) != nil {
			continue
		}
		now := formatTime(s.now())
		n, err := s.q.MarkRecoveryCodeUsed(ctx, generated.MarkRecoveryCodeUsedParams{
			UsedAt: &now,
			ID:     r.ID,
		})
		if err != nil {
			log.Printf("auth: recovery consume: %v", err)
			return "", ErrInvalidCode
		}
		if n == 0 {
			return "", ErrInvalidCode // burned by a racing attempt
		}
		return s.createSession(ctx)
	}
	return "", ErrInvalidCode
}

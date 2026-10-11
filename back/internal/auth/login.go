package auth

import (
	"context"
	"log"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"noted/internal/database/generated"
)

// Login maps every credential failure — wrong, spent, unenrolled, or an
// internal one — to ErrInvalidCode so one generic 401 answers all. Only a
// session-write failure propagates: there is no session to hide behind.
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

func (s *Service) loginTOTP(ctx context.Context, code string) (string, error) {
	ok, counter, err := s.ValidateCode(ctx, code)
	if err != nil {
		log.Printf("auth: internal: totp verify: %v", err)
		return "", ErrInvalidCode
	}

	if !ok {
		return "", ErrInvalidCode
	}

	if !s.acceptCounter(counter) {
		return "", ErrInvalidCode
	}

	return s.createSession(ctx, AUTH_METHOD_TOTP)
}

// acceptCounter is the replay guard: every counter accepted within
// REPLAY_WINDOW stays refused, even after another counter was accepted
// in between — an interleaved replay must not reset the guard. In-memory
// on purpose: a restart forgets it, a trade-off documented in SECURITY.md.
func (s *Service) acceptCounter(counter uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	for c, exp := range s.accepted {
		if !now.Before(exp) {
			delete(s.accepted, c)
		}
	}

	if _, seen := s.accepted[counter]; seen {
		return false
	}
	s.accepted[counter] = now.Add(REPLAY_WINDOW)

	return true
}

// loginRecovery scans the unused pool — salted bcrypt hashes cannot be
// looked up directly — and consumes the match with a conditional UPDATE:
// losing a race still yields the generic answer, never a second use.
func (s *Service) loginRecovery(ctx context.Context, code string) (string, error) {
	want := normalizeRecoveryCode(code)
	if want == "" {
		return "", ErrInvalidCode
	}

	rows, err := s.q.ListUnusedRecoveryCodes(ctx)
	if err != nil {
		log.Printf("auth: internal: recovery list: %v", err)
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
			log.Printf("auth: internal: recovery consume: %v", err)
			return "", ErrInvalidCode
		}

		if n == 0 {
			return "", ErrInvalidCode // burned by a racing attempt
		}

		return s.createSession(ctx, AUTH_METHOD_RECOVERY)
	}

	return "", ErrInvalidCode
}

package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"noted/internal/database/generated"
)

// createSession mints a 32-byte opaque token, stores only its SHA-256, and
// returns the token the handler puts in the cookie (SECURITY.md).
func (s *Service) createSession(ctx context.Context) (string, error) {
	token := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	now := s.now()
	err := s.q.CreateSession(ctx, generated.CreateSessionParams{
		ID:        uuid.NewString(),
		TokenHash: hashToken(token),
		CreatedAt: formatTime(now),
		ExpiresAt: formatTime(now.Add(SESSION_TTL)),
		LastSeen:  formatTime(now),
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

// hashToken is the at-rest form of a session cookie: SHA-256 hex, so a DB
// leak never yields a usable token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Validate resolves a cookie token to its sliding expiry. Empty, unknown or
// expired → ErrUnauthenticated. Otherwise last_seen is bumped and, when
// less than SESSION_REFRESH_AT remains, expires_at is re-issued — clamped
// to created_at + SESSION_ABSOLUTE_MAX so no request can extend forever
// (SECURITY.md).
func (s *Service) Validate(ctx context.Context, token string) (time.Time, error) {
	if token == "" {
		return time.Time{}, ErrUnauthenticated
	}
	row, err := s.q.GetSessionByTokenHash(ctx, hashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrUnauthenticated
	}
	if err != nil {
		log.Printf("auth: session lookup: %v", err)
		return time.Time{}, ErrUnauthenticated
	}
	now := s.now()
	expires, err := time.Parse(time.RFC3339, row.ExpiresAt)
	if err != nil {
		log.Printf("auth: session expiry parse: %v", err)
		return time.Time{}, ErrUnauthenticated
	}
	if !now.Before(expires) {
		return time.Time{}, ErrUnauthenticated
	}
	newExp := expires
	if expires.Sub(now) < SESSION_REFRESH_AT {
		newExp = now.Add(SESSION_TTL)
		createdAt, err := time.Parse(time.RFC3339, row.CreatedAt)
		if err != nil {
			log.Printf("auth: session created parse: %v", err)
		} else if limit := createdAt.Add(SESSION_ABSOLUTE_MAX); newExp.After(limit) {
			newExp = limit
		}
	}
	err = s.q.TouchSession(ctx, generated.TouchSessionParams{
		LastSeen:  formatTime(now),
		ExpiresAt: formatTime(newExp),
		ID:        row.ID,
	})
	if err != nil {
		// The row stays valid; the slide is retried on the next request.
		log.Printf("auth: session touch: %v", err)
	}
	return newExp, nil
}

// ValidateRequest is the guard's entry point: session cookie → Validate.
func (s *Service) ValidateRequest(r *http.Request) (time.Time, error) {
	c, err := r.Cookie(SESSION_COOKIE)
	if err != nil {
		return time.Time{}, ErrUnauthenticated
	}
	return s.Validate(r.Context(), c.Value)
}

// Logout deletes the row behind token. A missing token or row is not an
// error — the handler clears the cookie either way — but a failed delete is,
// because the session would otherwise survive server-side.
func (s *Service) Logout(ctx context.Context, token string) error {
	if err := s.q.DeleteSessionByTokenHash(ctx, hashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// PruneSessions removes rows whose expiry has passed. Called once at boot
// so a long-down instance does not keep dead sessions around.
func (s *Service) PruneSessions(ctx context.Context) error {
	return s.q.DeleteExpiredSessions(ctx, formatTime(s.now()))
}

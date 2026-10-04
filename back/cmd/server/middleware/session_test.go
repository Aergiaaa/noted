package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"noted/internal/auth"
	"noted/internal/database"
)

// seedSession inserts a session row directly (token hash computed here) so
// middleware tests do not pay bcrypt enrollment costs for a cookie.
func seedSession(t *testing.T, db *sql.DB, token string, expires, created time.Time) {
	t.Helper()
	sum := sha256.Sum256([]byte(token))
	_, err := db.Exec(
		`INSERT INTO sessions (id, token_hash, created_at, expires_at, last_seen) VALUES (?, ?, ?, ?, ?)`,
		"seed-"+token, hex.EncodeToString(sum[:]),
		created.UTC().Format(time.RFC3339),
		expires.UTC().Format(time.RFC3339),
		created.UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}
}

func newSessionService(t *testing.T) (*auth.Service, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return auth.New(db, ""), db
}

func TestRequireSession_validCookie_reachesHandler(t *testing.T) {
	svc, db := newSessionService(t)
	now := time.Now()
	seedSession(t, db, "good-token", now.Add(time.Hour), now)

	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusTeapot)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: "good-token"})
	rec := httptest.NewRecorder()

	RequireSession(svc)(next).ServeHTTP(rec, req)

	if !reached {
		t.Fatal("handler not reached with valid cookie")
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want teapot passthrough", rec.Code)
	}
}

func TestRequireSession_badCookie_401Envelope(t *testing.T) {
	svc, db := newSessionService(t)
	now := time.Now()

	for _, tc := range []struct {
		name, token string
	}{
		{"missing cookie", ""},
		{"unknown token", "nope"},
		{"expired session", "expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.token == "expired" {
				seedSession(t, db, "expired", now.Add(-time.Second), now.Add(-2*time.Hour))
			}
			reached := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })
			req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
			if tc.token != "" {
				req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: tc.token})
			}
			rec := httptest.NewRecorder()

			RequireSession(svc)(next).ServeHTTP(rec, req)

			if reached {
				t.Fatal("handler reached without a valid session")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}

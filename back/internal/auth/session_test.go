package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHashToken_isSHA256Hex(t *testing.T) {
	// sha256("test") — fixed vector so a silent algorithm change breaks it.
	const want = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if got := hashToken("test"); got != want {
		t.Fatalf("hashToken = %s, want %s", got, want)
	}
}

func TestCreateSession_storesOnlyTheHash(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }

	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	row, err := s.q.GetSessionByTokenHash(context.Background(), hashToken(token))
	if err != nil {
		t.Fatalf("lookup by hash: %v", err)
	}
	if row.TokenHash == token {
		t.Fatal("raw token stored in the database")
	}
	if want := formatTime(t0.Add(SESSION_TTL)); row.ExpiresAt != want {
		t.Fatalf("expires_at = %s, want %s", row.ExpiresAt, want)
	}
	if row.LastSeen != formatTime(t0) {
		t.Fatalf("last_seen = %s, want %s", row.LastSeen, formatTime(t0))
	}
	if row.AuthMethod != AUTH_METHOD_TOTP {
		t.Fatalf("auth_method = %q, want %q", row.AuthMethod, AUTH_METHOD_TOTP)
	}
}

func TestCreateSession_failure(t *testing.T) {
	s, db := newTestService(t, testEncKey(1))
	if _, err := db.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.createSession(context.Background(), AUTH_METHOD_TOTP); err == nil {
		t.Fatal("want error")
	}
}

func TestValidate_rejectsEmptyAndUnknownTokens(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	if _, err := s.Validate(context.Background(), ""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("empty token = %v, want ErrUnauthenticated", err)
	}
	if _, err := s.Validate(context.Background(), "no-such-token"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unknown token = %v, want ErrUnauthenticated", err)
	}
}

func TestValidate_reportsLookupFailure(t *testing.T) {
	s, db := newTestService(t, testEncKey(1))
	_ = db.Close()
	if _, err := s.Validate(context.Background(), "token"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("closed db = %v, want ErrUnauthenticated", err)
	}
}

func TestValidate_rejectsCorruptAndExpiredRows(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	t.Run("corrupt expiry", func(t *testing.T) {
		if _, err := s.db.Exec(`UPDATE sessions SET expires_at='garbage'`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Validate(context.Background(), token); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("= %v, want ErrUnauthenticated", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		if _, err := s.db.Exec(`UPDATE sessions SET expires_at=?`,
			formatTime(t0.Add(-time.Second))); err != nil {
			t.Fatal(err)
		}
		// now == t0 is strictly after t0-1s.
		if _, err := s.Validate(context.Background(), token); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("= %v, want ErrUnauthenticated", err)
		}
	})
}

func TestValidate_slidesLastSeenWithoutRefreshingExpiry(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	oneDay := t0.Add(24 * time.Hour)
	s.now = func() time.Time { return oneDay }
	got, err := s.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if want := t0.Add(SESSION_TTL); !got.Equal(want) {
		t.Fatalf("expiry = %s, want unrefreshed %s", got, want)
	}
	row, err := s.q.GetSessionByTokenHash(context.Background(), hashToken(token))
	if err != nil {
		t.Fatal(err)
	}
	if row.LastSeen != formatTime(oneDay) {
		t.Fatalf("last_seen = %s, want %s", row.LastSeen, formatTime(oneDay))
	}
	if row.ExpiresAt != formatTime(t0.Add(SESSION_TTL)) {
		t.Fatalf("expires_at = %s, want %s", row.ExpiresAt, formatTime(t0.Add(SESSION_TTL)))
	}
}

func TestValidate_refreshesNearExpiry(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	near := t0.Add(SESSION_TTL - 4*24*time.Hour) // 4d left < 7d refresh threshold
	s.now = func() time.Time { return near }
	got, err := s.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if want := near.Add(SESSION_TTL); !got.Equal(want) {
		t.Fatalf("expiry = %s, want refreshed %s", got, want)
	}
}

func TestValidate_clampsRefreshToAbsoluteCap(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	// Simulate slides already having pushed expiry close to the 90d cap:
	// created t0, expires t0+85d, now t0+84d → refresh would say now+30d
	// (t0+114d) but the absolute cap is t0+90d.
	if _, err := s.db.Exec(`UPDATE sessions SET expires_at=?`, formatTime(t0.Add(85*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return t0.Add(84 * 24 * time.Hour) }

	got, err := s.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if want := t0.Add(SESSION_ABSOLUTE_MAX); !got.Equal(want) {
		t.Fatalf("expiry = %s, want clamped %s", got, want)
	}
}

func TestValidate_unparsableCreatedAt_skipsClamp(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE sessions SET created_at='garbage', expires_at=?`,
		formatTime(t0.Add(29*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return t0.Add(28 * 24 * time.Hour) } // 1d left → refresh

	got, err := s.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if want := s.now().Add(SESSION_TTL); !got.Equal(want) {
		t.Fatalf("expiry = %s, want unclamped %s", got, want)
	}
}

func TestValidate_survivesTouchFailure(t *testing.T) {
	s, db := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if _, err := db.Exec(`CREATE TRIGGER block_touch BEFORE UPDATE ON sessions
		BEGIN SELECT RAISE(ABORT, 'blocked'); END`); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return t0.Add(time.Hour) }
	got, err := s.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if want := t0.Add(SESSION_TTL); !got.Equal(want) {
		t.Fatalf("expiry = %s, want %s", got, want)
	}
}

func TestValidateRequest_readsSessionCookie(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/session", nil)
	if _, _, err := s.ValidateRequest(req); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("no cookie = %v, want ErrUnauthenticated", err)
	}

	req.AddCookie(&http.Cookie{Name: SESSION_COOKIE, Value: token})
	got, _, err := s.ValidateRequest(req)
	if err != nil {
		t.Fatalf("with cookie: %v", err)
	}
	if got != token {
		t.Fatalf("ValidateRequest token = %q, want %q", got, token)
	}
}

func TestValidate_recentLastSeen_skipsTouch(t *testing.T) {
	// Steady state: same expiry, last_seen inside SESSION_TOUCH_INTERVAL —
	// the row must not be written at all.
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	s.now = func() time.Time { return t0.Add(SESSION_TOUCH_INTERVAL - time.Second) }
	if _, err := s.Validate(context.Background(), token); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	row, err := s.q.GetSessionByTokenHash(context.Background(), hashToken(token))
	if err != nil {
		t.Fatal(err)
	}
	if row.LastSeen != formatTime(t0) {
		t.Fatalf("last_seen = %s, want untouched %s", row.LastSeen, formatTime(t0))
	}
}

func TestCreateSession_pruneFailureTolerated(t *testing.T) {
	// A login must survive a failing sweep: seed an expired row so the
	// prune DELETE has a candidate, then block every DELETE.
	s, db := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	if _, err := db.Exec(
		`INSERT INTO sessions (id, token_hash, created_at, expires_at, last_seen)
		 VALUES ('old', 'stale-hash', ?, ?, ?)`,
		formatTime(t0.Add(-time.Hour)), formatTime(t0.Add(-time.Minute)), formatTime(t0),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER block_prune BEFORE DELETE ON sessions
		BEGIN SELECT RAISE(ABORT, 'blocked'); END`); err != nil {
		t.Fatal(err)
	}

	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession with failing prune: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
}

func TestValidateRequest_badSessionToken_unauthenticated(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SESSION_COOKIE, Value: "no-row"})

	token, _, err := s.ValidateRequest(req)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
}

func TestWriteSessionCookie_setsAndClears(t *testing.T) {
	set := httptest.NewRecorder()
	WriteSessionCookie(set, "tok", true)
	got := set.Result().Cookies()
	if len(got) != 1 {
		t.Fatalf("cookies = %d, want 1", len(got))
	}
	c := got[0]
	if c.Name != SESSION_COOKIE || c.Value != "tok" || c.Path != "/" {
		t.Fatalf("cookie = %+v", c)
	}
	if c.MaxAge != COOKIE_MAX_AGE_SEC || !c.HttpOnly || !c.Secure ||
		c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("attrs = MaxAge:%d HttpOnly:%v Secure:%v SameSite:%v",
			c.MaxAge, c.HttpOnly, c.Secure, c.SameSite)
	}

	clear := httptest.NewRecorder()
	WriteSessionCookie(clear, "", false)
	got = clear.Result().Cookies()
	if len(got) != 1 {
		t.Fatalf("clear cookies = %d, want 1", len(got))
	}
	if got[0].MaxAge >= 0 || got[0].Value != "" {
		t.Fatalf("clear cookie = %+v, want negative MaxAge + empty value", got[0])
	}
	if got[0].Secure {
		t.Fatal("Secure set with secure=false")
	}
}

func TestLogout_deletesTheSession(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if err := s.Logout(context.Background(), token); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := s.q.GetSessionByTokenHash(context.Background(), hashToken(token)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("row still present: %v", err)
	}
}

func TestLogout_reportsDeleteFailure(t *testing.T) {
	s, db := newTestService(t, testEncKey(1))
	if _, err := db.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(context.Background(), "token"); err == nil {
		t.Fatal("want error")
	}
}

func TestPruneSessions_removesExpiredRows(t *testing.T) {
	s, _ := newTestService(t, testEncKey(1))
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	token, err := s.createSession(context.Background(), AUTH_METHOD_TOTP)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	s.now = func() time.Time { return t0.Add(SESSION_TTL + time.Hour) }
	if err := s.PruneSessions(context.Background()); err != nil {
		t.Fatalf("PruneSessions: %v", err)
	}
	if _, err := s.q.GetSessionByTokenHash(context.Background(), hashToken(token)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired row survived: %v", err)
	}
}

func TestPruneSessions_reportsFailure(t *testing.T) {
	s, db := newTestService(t, testEncKey(1))
	_ = db.Close()
	if err := s.PruneSessions(context.Background()); err == nil {
		t.Fatal("want error")
	}
}

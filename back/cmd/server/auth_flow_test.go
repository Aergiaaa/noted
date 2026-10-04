package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"noted/internal/auth"
)

// postAuth issues a JSON POST against the app's router, optionally with
// the session cookie from a previous login.
func postAuth(t *testing.T, h http.Handler, path, body, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body == "" {
		rdr = strings.NewReader("")
	} else {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(http.MethodPost, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// sessionCookie pulls the noted_session value out of a login response.
func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SESSION_COOKIE {
			return c.Value
		}
	}
	t.Fatal("login response set no noted_session cookie")
	return ""
}

// TestAuthFlow_endToEnd walks the F12 critical path through the real
// router: guard → enroll → wrong code → live code → session → rotate
// recovery pool → dead/live codes → logout → rate limit.
func TestAuthFlow_endToEnd(t *testing.T) {
	app := newTestApp(t, Config{TOTPEncKey: cliEncKey})
	router := app.newRouter()

	// 1. Guard: no cookie yet.
	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("probe = %d, want 401", rec.Code)
	}

	// 2. Host-side enrollment (what `server enroll` does after the prompt).
	secret, oldCodes := hostEnroll(t, app)

	// 3. Wrong credential — generic 401.
	rec = postAuth(t, router, "/api/auth/login", `{"code":"nope"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code = %d, want 401", rec.Code)
	}

	// 4. Live TOTP → 200 + session cookie with the SECURITY.md attributes.
	live, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	rec = postAuth(t, router, "/api/auth/login", `{"code":"`+live+`"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("live code = %d, want 200", rec.Code)
	}
	cookie := sessionCookie(t, rec)
	sc := rec.Result().Cookies()[0]
	if !sc.HttpOnly || sc.SameSite != http.SameSiteLaxMode || sc.MaxAge != auth.COOKIE_MAX_AGE_SEC {
		t.Fatalf("cookie attrs = HttpOnly:%v SameSite:%v MaxAge:%d", sc.HttpOnly, sc.SameSite, sc.MaxAge)
	}

	// 5. Session probe with the cookie → 200 authenticated.
	req = httptest.NewRequest(http.MethodGet, "/api/session", nil)
	req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: cookie})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("session = %d, want 200", rec.Code)
	}
	var sess struct {
		Authenticated bool   `json:"authenticated"`
		ExpiresAt     string `json:"expires_at"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&sess); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if !sess.Authenticated || sess.ExpiresAt == "" {
		t.Fatalf("session body = %+v", sess)
	}

	// 6. Rotate the recovery pool (session required): 10 fresh codes.
	rec = postAuth(t, router, "/api/auth/recovery-codes", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate = %d, want 200", rec.Code)
	}
	var rotated struct {
		Codes []string `json:"recovery_codes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&rotated); err != nil {
		t.Fatalf("decode codes: %v", err)
	}
	if len(rotated.Codes) != auth.RECOVERY_CODE_COUNT {
		t.Fatalf("rotated codes = %d, want %d", len(rotated.Codes), auth.RECOVERY_CODE_COUNT)
	}

	// 7. Rotation burns the whole pool: an original code dies, a fresh one works.
	rec = postAuth(t, router, "/api/auth/login", `{"code":"`+oldCodes[0]+`"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("burned code = %d, want 401", rec.Code)
	}
	rec = postAuth(t, router, "/api/auth/login", `{"code":"`+rotated.Codes[0]+`"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("fresh recovery code = %d, want 200", rec.Code)
	}
	freshCookie := sessionCookie(t, rec)

	// 8. Logout clears server row + cookie; the probe with that token is
	// 401 again (the first login's session is a separate row and survives).
	rec = postAuth(t, router, "/api/auth/logout", "", freshCookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/session", nil)
	req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: freshCookie})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("probe after logout = %d, want 401", rec.Code)
	}

	// 9. The limiter counted every attempt so far (4): one more wrong code
	// is the 5th (401), the next is the 6th → 429 + Retry-After.
	rec = postAuth(t, router, "/api/auth/login", `{"code":"nope"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("attempt 5 = %d, want 401", rec.Code)
	}
	rec = postAuth(t, router, "/api/auth/login", `{"code":"nope"}`, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 6 = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("429 without Retry-After")
	}
}

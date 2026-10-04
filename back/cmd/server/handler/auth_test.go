package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"noted/internal/auth"
	"noted/internal/database"
)

// testEncKey is a fixed valid 32-byte TOTP_ENC_KEY (base64 of 'k'×32) so
// handler tests can enroll without reading the environment.
var testEncKey = base64.StdEncoding.EncodeToString([]byte(
	"kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk"))

// newTestAuthHandler wires a Handler + auth service over a migrated temp
// DB (the same composition main.wire builds) for endpoint tests.
func newTestAuthHandler(t *testing.T, secure bool) (*Handler, *auth.Service, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "handler.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := auth.New(db, testEncKey)
	return New(svc, secure), svc, db
}

// enrollTestHost commits an enrollment (10 bcrypt hashes — slow but real)
// and returns the TOTP secret for minting live codes.
func enrollTestHost(t *testing.T, svc *auth.Service) string {
	t.Helper()
	key, err := svc.Prep()
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	if _, err := svc.CommitEnrollment(context.Background(), key.Secret()); err != nil {
		t.Fatalf("CommitEnrollment: %v", err)
	}
	return key.Secret()
}

// seedSession inserts a session row directly so response-shape tests do
// not pay the enrollment cost just to obtain a cookie.
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

func postJSON(handlerFn func(http.ResponseWriter, *http.Request), path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handlerFn(rec, req)
	return rec
}

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) (status int, code string, fields map[string]string) {
	t.Helper()
	var body struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return rec.Code, body.Error.Code, body.Error.Fields
}

func TestLogin_missingCode_400WithFields(t *testing.T) {
	h, _, _ := newTestAuthHandler(t, false)
	rec := postJSON(h.Login, "/api/auth/login", `{"code":""}`)

	status, code, fields := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q, want 400 validation_error", status, code)
	}
	if fields["code"] != "required" {
		t.Fatalf("fields = %v, want code:required", fields)
	}
}

func TestLogin_malformedBody_400(t *testing.T) {
	h, _, _ := newTestAuthHandler(t, false)
	rec := postJSON(h.Login, "/api/auth/login", `not-json`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusBadRequest || code != "validation_error" {
		t.Fatalf("status=%d code=%q, want 400 validation_error", status, code)
	}
}

func TestLogin_wrongCode_401Generic(t *testing.T) {
	// No enrollment at all: every credential outcome must look identical.
	h, _, _ := newTestAuthHandler(t, false)
	rec := postJSON(h.Login, "/api/auth/login", `{"code":"123456"}`)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusUnauthorized || code != "unauthenticated" {
		t.Fatalf("status=%d code=%q, want 401 unauthenticated", status, code)
	}
	if body := rec.Body.String(); strings.Contains(body, "enrolled") {
		t.Fatalf("401 body leaks state: %s", body)
	}
}

func TestLogin_liveCode_200SetsSessionCookie(t *testing.T) {
	h, svc, _ := newTestAuthHandler(t, false)
	secret := enrollTestHost(t, svc)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	rec := postJSON(h.Login, "/api/auth/login", `{"code":"`+code+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || !body["ok"] {
		t.Fatalf("body = %v, %v; want {ok:true}", body, err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != auth.SESSION_COOKIE || c.Value == "" {
		t.Fatalf("cookie = %q/%q, want %s with a value", c.Name, c.Value, auth.SESSION_COOKIE)
	}
	if !c.HttpOnly || c.Path != "/" || c.MaxAge != auth.COOKIE_MAX_AGE_SEC {
		t.Fatalf("cookie attrs = HttpOnly:%v Path:%q MaxAge:%d", c.HttpOnly, c.Path, c.MaxAge)
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.Secure {
		t.Fatal("Secure set in dev (APP_ENV != prod)")
	}
}

func TestLogin_prodHandler_setsSecureCookie(t *testing.T) {
	h, svc, _ := newTestAuthHandler(t, true)
	secret := enrollTestHost(t, svc)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	rec := postJSON(h.Login, "/api/auth/login", `{"code":"`+code+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if c := rec.Result().Cookies()[0]; !c.Secure {
		t.Fatal("Secure missing in prod")
	}
}

func TestLogin_sessionWriteFailure_500(t *testing.T) {
	h, svc, db := newTestAuthHandler(t, false)
	secret := enrollTestHost(t, svc)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatal(err)
	}

	rec := postJSON(h.Login, "/api/auth/login", `{"code":"`+code+`"}`)

	status, codeName, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || codeName != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, codeName)
	}
}

func TestSession_401WithoutValidCookie(t *testing.T) {
	h, _, db := newTestAuthHandler(t, false)
	seedSession(t, db, "expired-token", time.Now().Add(-time.Minute), time.Now().Add(-time.Hour))

	for _, token := range []string{"", "unknown", "expired-token"} {
		req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: token})
		}
		rec := httptest.NewRecorder()
		h.Session(rec, req)

		status, code, _ := decodeErr(t, rec)
		if status != http.StatusUnauthorized || code != "unauthenticated" {
			t.Fatalf("token %q: status=%d code=%q, want 401", token, status, code)
		}
	}
}

func TestSession_200ReturnsExpiryAndSlidesCookie(t *testing.T) {
	h, _, db := newTestAuthHandler(t, false)
	now := time.Now()
	seedSession(t, db, "live-token", now.Add(30*24*time.Hour), now)

	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: "live-token"})
	rec := httptest.NewRecorder()
	h.Session(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Authenticated bool   `json:"authenticated"`
		ExpiresAt     string `json:"expires_at"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Authenticated {
		t.Fatal("authenticated = false")
	}
	if _, err := time.Parse(time.RFC3339, body.ExpiresAt); err != nil {
		t.Fatalf("expires_at %q not RFC3339: %v", body.ExpiresAt, err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.SESSION_COOKIE {
		t.Fatalf("sliding cookie missing: %+v", cookies)
	}
	if cookies[0].Value != "live-token" {
		t.Fatalf("re-issued cookie value = %q, want same token", cookies[0].Value)
	}
}

func TestLogout_204DeletesRowAndClearsCookie(t *testing.T) {
	h, _, db := newTestAuthHandler(t, false)
	now := time.Now()
	seedSession(t, db, "out-token", now.Add(time.Hour), now)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: "out-token"})
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("clearing cookie missing/wrong: %+v", cookies)
	}
}

func TestLogout_withoutCookie_204(t *testing.T) {
	h, _, _ := newTestAuthHandler(t, false)
	rec := httptest.NewRecorder()
	h.Logout(rec, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestLogout_deleteFailure_500(t *testing.T) {
	h, _, db := newTestAuthHandler(t, false)
	if _, err := db.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: "x"})
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500", status, code)
	}
}

func TestRecoveryCodes_200RotatesOnce(t *testing.T) {
	h, svc, _ := newTestAuthHandler(t, false)
	enrollTestHost(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/recovery-codes", nil)
	rec := httptest.NewRecorder()
	h.RecoveryCodes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Codes []string `json:"recovery_codes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Codes) != auth.RECOVERY_CODE_COUNT {
		t.Fatalf("codes = %d, want %d", len(body.Codes), auth.RECOVERY_CODE_COUNT)
	}
	for _, c := range body.Codes {
		if len(c) != 9 || !strings.Contains(c, "-") {
			t.Fatalf("code %q does not look like XXXX-XXXX", c)
		}
	}
}

func TestRecoveryCodes_internalFailure_500(t *testing.T) {
	h, svc, db := newTestAuthHandler(t, false)
	enrollTestHost(t, svc)
	if _, err := db.Exec(`DROP TABLE recovery_codes`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/recovery-codes", nil)
	rec := httptest.NewRecorder()
	h.RecoveryCodes(rec, req)

	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500", status, code)
	}
}

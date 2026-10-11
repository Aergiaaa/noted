package main

import (
	"context"
	"encoding/base64"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"noted/internal/auth"
	"noted/internal/database"
)

// cliEncKey is a fixed valid 32-byte TOTP_ENC_KEY for CLI tests.
var cliEncKey = base64.StdEncoding.EncodeToString([]byte(
	"kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk"))

// codeLine matches one printed recovery code (two-space indent, grouped
// Crockford symbols).
var codeLine = regexp.MustCompile(`(?m)^  [0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$`)

// newCLIApp builds an app whose subcommands operate on a fresh temp DB.
func newCLIApp(t *testing.T) *app {
	t.Helper()
	return newApp(Config{
		DBPath:     filepath.Join(t.TempDir(), "cli.db"),
		TOTPEncKey: cliEncKey,
	})
}

// openAuth opens the app's DB for state assertions.
func openAuth(t *testing.T, a *app) *auth.Service {
	t.Helper()
	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return auth.New(db, a.conf.TOTPEncKey)
}

// feedStdin points os.Stdin at a pipe pre-loaded with data; the returned
// func restores it.
func feedStdin(t *testing.T, data string) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, data); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	return func() {
		os.Stdin = old
		_ = r.Close()
	}
}

// runCLIStreams runs a.cli with os.Stdout/os.Stderr swapped for temp files
// and returns their contents plus (handled, code).
func runCLIStreams(t *testing.T, a *app, args []string) (out, errOut string, code int, handled bool) {
	t.Helper()
	outF, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errF, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outF, errF
	handled, code = a.cli(args)
	os.Stdout, os.Stderr = oldOut, oldErr
	_ = outF.Close()
	_ = errF.Close()

	ob, err := os.ReadFile(outF.Name())
	if err != nil {
		t.Fatal(err)
	}
	eb, err := os.ReadFile(errF.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(ob), string(eb), code, handled
}

// runCLI executes one subcommand with pre-written stdin lines (closed
// after the write, so an unexpected prompt sees EOF and types an empty
// code), capturing stdout/stderr around the call.
func runCLI(t *testing.T, a *app, args []string, stdin ...string) (out, errOut string, code int) {
	t.Helper()
	if len(stdin) > 0 {
		restore := feedStdin(t, strings.Join(stdin, "\n")+"\n")
		defer restore()
	}
	out, errOut, code, handled := runCLIStreams(t, a, args)
	if !handled {
		t.Fatalf("cli(%v) not handled, want subcommand", args)
	}
	return out, errOut, code
}

// runEnrollLive drives `server enroll` the way an operator would: start
// it, wait until the otpauth URI is on stdout, mint the matching live
// code from the printed secret, then answer the prompt.
func runEnrollLive(t *testing.T, a *app, extraArgs ...string) (out, errOut string, code int) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldIn; _ = r.Close(); _ = w.Close() }()

	outF, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errF, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outF, errF

	type result struct {
		handled bool
		code    int
	}
	done := make(chan result, 1)
	args := append([]string{"enroll"}, extraArgs...)
	go func() {
		h, c := a.cli(args)
		done <- result{h, c}
	}()

	uri := waitForURI(t, outF.Name())
	secret := secretFromURI(t, uri)
	live, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := io.WriteString(w, live+"\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	res := <-done
	os.Stdout, os.Stderr = oldOut, oldErr
	_ = outF.Close()
	_ = errF.Close()

	ob, err := os.ReadFile(outF.Name())
	if err != nil {
		t.Fatal(err)
	}
	eb, err := os.ReadFile(errF.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !res.handled {
		t.Fatal("enroll not handled")
	}
	return string(ob), string(eb), res.code
}

// waitForURI polls captured stdout for the URI line printed just before
// the prompt; missing it means the command failed early.
func waitForURI(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil {
			if i := strings.Index(string(b), "URI: "); i >= 0 {
				rest := string(b)[i+len("URI: "):]
				if j := strings.IndexByte(rest, '\n'); j >= 0 {
					return rest[:j]
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("enroll never printed the URI (failed before the prompt?)")
	return ""
}

// secretFromURI pulls the secret= parameter out of an otpauth:// URI.
func secretFromURI(t *testing.T, uri string) string {
	t.Helper()
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parse URI %q: %v", uri, err)
	}
	secret := u.Query().Get("secret")
	if secret == "" {
		t.Fatalf("URI %q has no secret", uri)
	}
	return secret
}

// hostEnroll commits an enrollment directly (host-side, like the CLI
// would after a live code) and returns the secret + codes.
func hostEnroll(t *testing.T, a *app) (string, []string) {
	t.Helper()
	svc := openAuth(t, a)
	key, err := svc.Prep()
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	codes, err := svc.CommitEnrollment(context.Background(), key.Secret())
	if err != nil {
		t.Fatalf("CommitEnrollment: %v", err)
	}
	return key.Secret(), codes
}

// --- server enroll ---

func TestEnroll_happyPath_persistsAndPrintsRecoveryCodes(t *testing.T) {
	a := newCLIApp(t)
	out, errOut, code := runEnrollLive(t, a)

	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty on success", errOut)
	}
	for _, want := range []string{"enrolled — recovery codes", "otpauth://totp/Noted:me", "URI: "} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q", want)
		}
	}
	if n := len(codeLine.FindAllString(out, -1)); n != auth.RECOVERY_CODE_COUNT {
		t.Errorf("printed codes = %d, want %d", n, auth.RECOVERY_CODE_COUNT)
	}
	if strings.Contains(out, "SVG saved") {
		t.Error("SVG line without --qr")
	}

	enrolled, err := openAuth(t, a).Enrolled(context.Background())
	if err != nil || !enrolled {
		t.Fatalf("Enrolled = %v, %v; want true, nil", enrolled, err)
	}
}

func TestEnroll_qrFlag_writesSVG(t *testing.T) {
	a := newCLIApp(t)
	svg := filepath.Join(t.TempDir(), "qr.svg")
	out, errOut, code := runEnrollLive(t, a, "--qr", svg)

	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "SVG saved to "+svg) {
		t.Fatalf("stdout missing SVG confirmation: %s", out)
	}
	b, err := os.ReadFile(svg)
	if err != nil {
		t.Fatalf("read svg: %v", err)
	}
	if !strings.Contains(string(b), "<svg") {
		t.Fatalf("svg file does not look like SVG: %.80s", b)
	}
}

func TestEnroll_qrWriteFailure_exitsBeforePrompt(t *testing.T) {
	a := newCLIApp(t)
	bad := filepath.Join(t.TempDir(), "no-such-dir", "deep", "qr.svg")
	_, errOut, code := runCLI(t, a, []string{"enroll", "--qr", bad})

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "write") {
		t.Fatalf("stderr = %q, want write failure", errOut)
	}
	enrolled, err := openAuth(t, a).Enrolled(context.Background())
	if err != nil || enrolled {
		t.Fatalf("Enrolled = %v, %v; want false, nil", enrolled, err)
	}
}

func TestEnroll_missingKey_failsBeforePrompt(t *testing.T) {
	a := newApp(Config{DBPath: filepath.Join(t.TempDir(), "cli.db")})
	_, errOut, code := runCLI(t, a, []string{"enroll"})

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "TOTP_ENC_KEY") {
		t.Fatalf("stderr = %q, want TOTP_ENC_KEY message", errOut)
	}
}

func TestEnroll_alreadyEnrolled_exits1(t *testing.T) {
	a := newCLIApp(t)
	hostEnroll(t, a)

	_, errOut, code := runCLI(t, a, []string{"enroll"})

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "already enrolled") {
		t.Fatalf("stderr = %q, want already-enrolled message", errOut)
	}
}

func TestEnroll_badUsage_exits2(t *testing.T) {
	a := newCLIApp(t)
	for _, args := range [][]string{
		{"enroll", "--qr"},
		{"enroll", "extra", "args", "here"},
		{"enroll", "--qr", "a.svg", "b"},
	} {
		if _, errOut, code := runCLI(t, a, args); code != 2 {
			t.Errorf("args %v: exit = %d, want 2 (stderr %q)", args, code, errOut)
		}
	}
}

func TestEnroll_dbOpenError_exits1(t *testing.T) {
	base := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(base, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newApp(Config{DBPath: filepath.Join(base, "cli.db"), TOTPEncKey: cliEncKey})

	_, errOut, code := runCLI(t, a, []string{"enroll"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if errOut == "" {
		t.Fatal("stderr empty, want open error")
	}
}

func TestEnroll_threeInvalidAttempts_persistsNothing(t *testing.T) {
	a := newCLIApp(t)
	// Non-digit inputs can never match a TOTP code, so the misses are
	// deterministic without knowing the in-memory secret.
	_, errOut, code := runCLI(t, a, []string{"enroll"}, "nope", "bad", "junk")

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if n := strings.Count(errOut, "invalid code"); n != 3 {
		t.Fatalf("invalid-code lines = %d, want 3 (%q)", n, errOut)
	}
	if !strings.Contains(errOut, "too many failed attempts") {
		t.Fatalf("stderr = %q, want give-up message", errOut)
	}
	enrolled, err := openAuth(t, a).Enrolled(context.Background())
	if err != nil || enrolled {
		t.Fatalf("Enrolled after failed attempts = %v, %v; want false, nil", enrolled, err)
	}
}

func TestEnroll_commitFailure_exits1(t *testing.T) {
	a := newCLIApp(t)
	// Enrollment inserts first, then the pool: dropping only the pool
	// table fails the commit after the prompt was answered correctly.
	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE recovery_codes`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	_, errOut, code := runEnrollLive(t, a)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, errOut)
	}
	if !strings.Contains(errOut, "insert recovery code") {
		t.Fatalf("stderr = %q, want recovery insert failure", errOut)
	}
}

// --- server reset-auth ---

func TestResetAuth_requiresYes(t *testing.T) {
	a := newCLIApp(t)
	for _, args := range [][]string{
		{"reset-auth"},
		{"reset-auth", "--force"},
		{"reset-auth", "--yes", "extra"},
	} {
		if _, errOut, code := runCLI(t, a, args); code != 2 {
			t.Errorf("args %v: exit = %d, want 2 (%q)", args, code, errOut)
		} else if !strings.Contains(errOut, "usage: server reset-auth --yes") {
			t.Errorf("args %v: stderr = %q, want usage", args, errOut)
		}
	}
}

func TestResetAuth_yes_wipesEnrollment(t *testing.T) {
	a := newCLIApp(t)
	hostEnroll(t, a)

	out, errOut, code := runCLI(t, a, []string{"reset-auth", "--yes"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (%s)", code, errOut)
	}
	if !strings.Contains(out, "auth state wiped") {
		t.Fatalf("stdout = %q, want confirmation", out)
	}
	enrolled, err := openAuth(t, a).Enrolled(context.Background())
	if err != nil || enrolled {
		t.Fatalf("Enrolled = %v, %v; want false, nil", enrolled, err)
	}
}

func TestResetAuth_dbOpenError_exits1(t *testing.T) {
	base := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(base, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newApp(Config{DBPath: filepath.Join(base, "cli.db"), TOTPEncKey: cliEncKey})

	if _, _, code := runCLI(t, a, []string{"reset-auth", "--yes"}); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestResetAuth_wipeFailure_exits1(t *testing.T) {
	a := newCLIApp(t)
	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE enrollment`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	_, errOut, code := runCLI(t, a, []string{"reset-auth", "--yes"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "clear enrollment") {
		t.Fatalf("stderr = %q, want clear-enrollment failure", errOut)
	}
}

func TestEnroll_enrolledCheckFailure_exits1(t *testing.T) {
	a := newCLIApp(t)
	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE enrollment`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	_, errOut, code := runCLI(t, a, []string{"enroll"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "enroll: ") {
		t.Fatalf("stderr = %q, want enroll error line", errOut)
	}
}

package main

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"noted/internal/database"
)

var newKeyShape = regexp.MustCompile(`^[A-Za-z0-9+/=]{44}$`)

// wiredCLIApp builds a newCLIApp and wires it so a.auth is set.
func wiredCLIApp(t *testing.T) *app {
	t.Helper()
	a := newCLIApp(t)
	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a.wire(db)
	return a
}

func TestCli_noArgs_fallsThroughToServe(t *testing.T) {
	handled, code := newApp(Config{}).cli(nil)
	if handled || code != 0 {
		t.Fatalf("cli(nil) = %v, %d; want false, 0 (serve)", handled, code)
	}
}

func TestCli_rotateEncKey_noEnrollment_exits1(t *testing.T) {
	a := wiredCLIApp(t)
	out, errOut, code := runCLI(t, a, []string{"rotate-enc-key"})

	if code != 1 {
		t.Fatalf("rotate-enc-key without enrollment: exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "not enrolled") {
		t.Fatalf("stderr = %q, want not-enrolled message", errOut)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want nothing", out)
	}
}

func TestCli_rotateEncKey_rotatesAndPrintsNewKey(t *testing.T) {
	a := wiredCLIApp(t)
	secret, _ := hostEnroll(t, a)

	out, _, code := runCLI(t, a, []string{"rotate-enc-key"})
	if code != 0 {
		t.Fatalf("rotate-enc-key: exit = %d, want 0", code)
	}
	if !newKeyShape.MatchString(strings.TrimSpace(out)) {
		t.Fatalf("stdout = %q, want base64 32-byte key", out)
	}

	// TOTP still validates against the re-sealed secret using the original
	// enrolled secret. a.auth already has the new key.
	code6, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if ok, _, err := a.auth.ValidateCode(context.Background(), code6); err != nil || !ok {
		t.Fatalf("ValidateCode after rotation = %v, %v; want true, nil", ok, err)
	}
}

func TestCli_rotateEncKey_badUsage_exits2(t *testing.T) {
	a := wiredCLIApp(t)
	out, errOut, code := runCLI(t, a, []string{"rotate-enc-key", "extra"})
	if !strings.Contains(out+errOut, "usage: server") {
		t.Fatalf("output = %q/%q, want usage line", out, errOut)
	}
	if code != 2 {
		t.Fatalf("rotate-enc-key with extra args: exit = %d, want 2", code)
	}
}

func TestCli_rotateEncKey_noKey_exits1(t *testing.T) {
	a := wiredCLIApp(t)
	a.conf.TOTPEncKey = ""
	_, errOut, code := runCLI(t, a, []string{"rotate-enc-key"})
	if code != 1 {
		t.Fatalf("rotate-enc-key without key: exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "TOTP_ENC_KEY is not set") {
		t.Fatalf("stderr = %q, want TOTP_ENC_KEY message", errOut)
	}
}

// TestCli_unknownCommand_usageAndExit2 keeps testing that the usage line
// lists rotate-enc-key.
func TestCli_unknownCommand_usageAndExit2(t *testing.T) {
	a := newApp(Config{})
	handled, code := a.cli([]string{"drop-everything"})

	if !handled || code != 2 {
		t.Fatalf("cli = %v, %d; want true, 2", handled, code)
	}
	_, errOut, _ := runCLI(t, a, []string{"definitely-not-a-command"})
	if !strings.Contains(errOut, "rotate-enc-key") {
		t.Fatalf("stderr = %q, want usage line listing rotate-enc-key", errOut)
	}
}

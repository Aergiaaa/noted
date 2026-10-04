package main

import (
	"strings"
	"testing"
)

func TestCli_noArgs_fallsThroughToServe(t *testing.T) {
	handled, code := newApp(Config{}).cli(nil)
	if handled || code != 0 {
		t.Fatalf("cli(nil) = %v, %d; want false, 0 (serve)", handled, code)
	}
}

func TestCli_unknownCommand_usageAndExit2(t *testing.T) {
	a := newApp(Config{})
	handled, code := a.cli([]string{"drop-everything"})

	if !handled || code != 2 {
		t.Fatalf("cli = %v, %d; want true, 2", handled, code)
	}
	_, errOut, _ := runCLI(t, a, []string{"definitely-not-a-command"})
	if !strings.Contains(errOut, "usage: server") {
		t.Fatalf("stderr = %q, want usage line", errOut)
	}
}

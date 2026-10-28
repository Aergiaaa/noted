package main

import (
	"context"
	"fmt"
	"os"
)

// cli runs the host-only subcommands — host access *is* the enrollment
// credential (SECURITY.md), so these never listen on a socket.
// handled=false means "no subcommand matched, serve"; code is the exit status.
func (a *app) cli(args []string) (handled bool, code int) {
	if len(args) == 0 {
		return false, 0
	}
	switch args[0] {
	case "enroll":
		return true, a.runEnroll(args[1:])
	case "reset-auth":
		return true, a.runResetAuth(args[1:])
	case "rotate-enc-key":
		return true, a.runRotateEncKey(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: server [enroll [--qr FILE.svg] | reset-auth --yes | rotate-enc-key]")
		return true, 2
	}
}

// runRotateEncKey re-encrypts the stored TOTP secret with a new random key.
// The current key (TOTP_ENC_KEY from the environment) decrypts the existing
// secret; the new key seals it. The operator must update TOTP_ENC_KEY in
// .env and restart for the change to persist across restarts.
func (a *app) runRotateEncKey(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: server rotate-enc-key")
		return 2
	}

	if a.conf.TOTPEncKey == "" {
		fmt.Fprintln(os.Stderr, "rotate-enc-key: TOTP_ENC_KEY is not set")
		return 1
	}

	newKey, err := a.auth.RotateEncKey(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotate-enc-key: %v\n", err)
		return 1
	}

	fmt.Println(newKey)
	return 0
}

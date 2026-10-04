package main

import (
	"fmt"
	"os"
)

// cli dispatches the host-only subcommands — SECURITY.md: host access *is*
// the enrollment credential, so these never listen on a socket. handled=false
// means "no subcommand matched, serve"; code is the process exit status.
func (a *app) cli(args []string) (handled bool, code int) {
	if len(args) == 0 {
		return false, 0
	}
	switch args[0] {
	case "enroll":
		return true, a.runEnroll(args[1:])
	case "reset-auth":
		return true, a.runResetAuth(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: server [enroll [--qr FILE.svg] | reset-auth --yes]")
		return true, 2
	}
}

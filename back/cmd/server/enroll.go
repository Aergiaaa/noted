package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"noted/internal/auth"
	"noted/internal/database"
)

// MAX_ENROLL_ATTEMPTS caps the live-code prompt: the secret is generated
// in memory and nothing is persisted until a correct code proves the
// authenticator app is set up (SECURITY.md).
const MAX_ENROLL_ATTEMPTS = 3

// runEnroll is `server enroll [--qr FILE.svg]`: print an ASCII QR (plus
// the otpauth URI) for the authenticator app, verify one live 6-digit
// code, then persist the sealed secret and the recovery pool in one
// transaction. Wrong key, already-enrolled and bad usage fail before the
// prompt; failed attempts leave no trace.
func (a *app) runEnroll(args []string) int {
	var qrPath string
	switch {
	case len(args) == 0:
	case len(args) == 2 && args[0] == "--qr":
		qrPath = args[1]
	default:
		fmt.Fprintln(os.Stderr, "usage: server enroll [--qr FILE.svg]")
		return 2
	}

	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "enroll: %v\n", err)
		return 1
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	svc := auth.New(db, a.conf.TOTPEncKey)
	enrolled, err := svc.Enrolled(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "enroll: %v\n", err)
		return 1
	}
	if enrolled {
		fmt.Fprintln(os.Stderr, "already enrolled — run `server reset-auth --yes` first")
		return 1
	}
	key, err := svc.Prep()
	if err != nil {
		fmt.Fprintf(os.Stderr, "enroll: %v\n", err)
		return 1
	}

	printQR(key.URL())
	if qrPath != "" {
		if err := writeQRSVG(key.URL(), qrPath); err != nil {
			fmt.Fprintf(os.Stderr, "enroll: write %s: %v\n", qrPath, err)
			return 1
		}
		fmt.Printf("SVG saved to %s\n", qrPath)
	}
	fmt.Printf("URI: %s\n", key.URL())
	fmt.Println("Scan the QR with your authenticator app, then type a live code:")

	for attempt := 1; attempt <= MAX_ENROLL_ATTEMPTS; attempt++ {
		fmt.Printf("code (%d/%d): ", attempt, MAX_ENROLL_ATTEMPTS)
		var typed string
		_, _ = fmt.Scanln(&typed) // EOF/typing errors surface as an empty code
		// The secret is still in memory here — nothing is in the DB for
		// ValidateCode to read — so the window check runs against key.
		if ok, _ := svc.ValidateCandidate(strings.TrimSpace(typed), key.Secret()); ok {
			codes, cerr := svc.CommitEnrollment(ctx, key.Secret())
			if cerr != nil {
				fmt.Fprintf(os.Stderr, "enroll: %v\n", cerr)
				return 1
			}
			fmt.Println("enrolled — recovery codes (store offline, shown once):")
			for _, c := range codes {
				fmt.Printf("  %s\n", c)
			}
			return 0
		}
		fmt.Fprintln(os.Stderr, "invalid code")
	}
	fmt.Fprintln(os.Stderr, "too many failed attempts — nothing was persisted")
	return 1
}

// runResetAuth is `server reset-auth --yes`: wipe enrollment, recovery
// codes and sessions so the host can enroll fresh. --yes is required —
// a typo must not destroy the only credential.
func (a *app) runResetAuth(args []string) int {
	if len(args) != 1 || args[0] != "--yes" {
		fmt.Fprintln(os.Stderr, "usage: server reset-auth --yes   (wipes enrollment, recovery codes and sessions)")
		return 2
	}

	db, err := database.Open(a.conf.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reset-auth: %v\n", err)
		return 1
	}
	defer func() { _ = db.Close() }()

	if err := auth.New(db, a.conf.TOTPEncKey).ResetAuth(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "reset-auth: %v\n", err)
		return 1
	}
	fmt.Println("auth state wiped — run `server enroll` to re-arm")
	return 0
}

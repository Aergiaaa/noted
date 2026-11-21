package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"noted/internal/auth"
	"noted/internal/database"
)

// MAX_ENROLL_ATTEMPTS caps the live-code prompt: the secret lives in
// memory and nothing is persisted until a correct code proves the
// authenticator app is set up (SECURITY.md).
const MAX_ENROLL_ATTEMPTS = 3

// runEnroll prints the QR, verifies one live code, then commits the sealed
// secret and the recovery pool in one transaction — failed attempts leave
// no trace. `--secret BASE32` skips the live-code proof: the operator (or
// the E2E stack) already owns the secret, so proving an authenticator can
// derive it adds nothing (SECURITY.md).
func (a *app) runEnroll(args []string) int {
	var qrPath, secret string
	secretSet := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--qr":
			if i+1 >= len(args) {
				return enrollUsage()
			}
			i++
			qrPath = args[i]
		case "--secret":
			if i+1 >= len(args) {
				return enrollUsage()
			}
			i++
			secret, secretSet = args[i], true
		default:
			return enrollUsage()
		}
	}

	if secretSet {
		normalized, err := auth.NormalizeSecret(secret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "enroll: %v\n", err)
			return 2
		}
		secret = normalized
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

	uri := ""
	candidateSecret := secret
	if !secretSet {
		key, err := svc.Prep()
		if err != nil {
			fmt.Fprintf(os.Stderr, "enroll: %v\n", err)
			return 1
		}
		uri, candidateSecret = key.URL(), key.Secret()
	} else {
		if err := svc.CheckEncKey(); err != nil {
			fmt.Fprintf(os.Stderr, "enroll: %v\n", err)
			return 1
		}
		uri = auth.EnrollURI(candidateSecret)
	}

	printQR(uri)
	if qrPath != "" {
		if err := writeQRSVG(uri, qrPath); err != nil {
			fmt.Fprintf(os.Stderr, "enroll: write %s: %v\n", qrPath, err)
			return 1
		}
		fmt.Printf("SVG saved to %s\n", qrPath)
	}
	fmt.Printf("URI: %s\n", uri)

	if secretSet {
		return commitEnrollment(ctx, svc, secret)
	}

	fmt.Println("Scan the QR with your authenticator app, then type a live code:")

	for attempt := 1; attempt <= MAX_ENROLL_ATTEMPTS; attempt++ {
		fmt.Printf("code (%d/%d): ", attempt, MAX_ENROLL_ATTEMPTS)
		var typed string
		_, _ = fmt.Scanln(&typed) // EOF/typing errors surface as an empty code
		// The secret is still only in memory here, so the window check runs
		// against candidateSecret — ValidateCode would find no enrollment row.
		if ok, _ := svc.ValidateCandidate(strings.TrimSpace(typed), candidateSecret); ok {
			return commitEnrollment(ctx, svc, candidateSecret)
		}
		fmt.Fprintln(os.Stderr, "invalid code")
	}
	fmt.Fprintln(os.Stderr, "too many failed attempts — nothing was persisted")
	return 1
}

func enrollUsage() int {
	fmt.Fprintln(os.Stderr, "usage: server enroll [--qr FILE.svg] [--secret BASE32]")
	return 2
}

// commitEnrollment persists the secret + recovery pool and prints the
// codes once; both enroll paths end here.
func commitEnrollment(ctx context.Context, svc *auth.Service, secret string) int {
	codes, err := svc.CommitEnrollment(ctx, secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "enroll: %v\n", err)
		return 1
	}

	fmt.Println("enrolled — recovery codes (store offline, shown once):")
	for _, c := range codes {
		fmt.Printf("  %s\n", c)
	}
	return 0
}

// runResetAuth requires --yes: a typo must not destroy the only credential.
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

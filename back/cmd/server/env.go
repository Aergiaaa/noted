package main

import (
	"log"
	"os"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// warnStaleDataPath surfaces the F2 config rename: DEPLOYMENT.md's
// checklist moves DATA_PATH to DB_PATH, and a stale key is otherwise
// ignored silently — boot falls back to the default path, which prod's
// read-only rootfs rejects (EROFS) into a crash loop.
func warnStaleDataPath() {
	if os.Getenv("DATA_PATH") != "" {
		log.Print("DATA_PATH was renamed to DB_PATH; DATA_PATH is ignored")
	}
}

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

// warnStaleDataPath surfaces the F2 config rename: a stale DATA_PATH key
// is otherwise ignored silently, boot falls back to the default path, and
// prod's read-only rootfs rejects it (EROFS) into a crash loop.
func warnStaleDataPath() {
	if os.Getenv("DATA_PATH") != "" {
		log.Print("DATA_PATH was renamed to DB_PATH; DATA_PATH is ignored")
	}
}

package service

import (
	"strings"
	"unicode"
)

// sanitizeLine: strip every control character (tabs included) and trim
// the edges — a title is a single opaque line (DOMAIN.md lengths still
// apply after sanitizing).
func sanitizeLine(s string) string {
	return strings.TrimSpace(strings.Map(dropControl, s))
}

// sanitizeBody: newlines and tabs survive; CRLF and lone CR normalize to
// LF first so stored bodies never carry Windows line endings.
func sanitizeBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(strings.Map(dropControlKeepBreaks, s))
}

// sanitizeDescription: nil in, nil out; a body that reduces to "" (only
// controls/whitespace) becomes "no description" rather than "".
func sanitizeDescription(d *string) *string {
	if d == nil {
		return nil
	}
	clean := sanitizeBody(*d)
	if clean == "" {
		return nil
	}
	return &clean
}

func dropControl(r rune) rune {
	if unicode.IsControl(r) {
		return -1
	}
	return r
}

func dropControlKeepBreaks(r rune) rune {
	if unicode.IsControl(r) && r != '\n' && r != '\t' {
		return -1
	}
	return r
}

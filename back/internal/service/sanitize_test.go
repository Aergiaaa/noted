package service

import "testing"

func TestSanitizeLine_stripsControlsAndTrims(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
	}{
		{"control chars and tab stripped", "Den\x00t\tist ", "Dentist"},
		{"esc and bell stripped", "esc\x1bape\x07", "escape"},
		{"control only becomes empty", "\x00\x07\t\n", ""},
		{"edges trimmed", "  hello  ", "hello"},
		{"plain passthrough", "Dentist", "Dentist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeLine(tt.in); got != tt.want {
				t.Errorf("sanitizeLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeBody_normalizesCRAndKeepsBreaks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
	}{
		{"CRLF normalizes to LF, controls stripped", "line1\r\nline2\x07", "line1\nline2"},
		{"lone CR normalizes to LF", "a\rb", "a\nb"},
		{"newlines and tabs survive", "keep\nlines\tand tabs", "keep\nlines\tand tabs"},
		{"surrounding whitespace trimmed", "\n\tpadded body\t\n", "padded body"},
		{"control only becomes empty", "\x00\x07\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeBody(tt.in); got != tt.want {
				t.Errorf("sanitizeBody(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeDescription_nilInNilOutAndEmptyBecomesNil(t *testing.T) {
	t.Parallel()
	if got := sanitizeDescription(nil); got != nil {
		t.Errorf("sanitizeDescription(nil) = %v, want nil", got)
	}
	controlOnly := "\x00\x07"
	if got := sanitizeDescription(&controlOnly); got != nil {
		t.Errorf("sanitizeDescription(control-only) = %v, want nil", got)
	}
	dirty := "clean\x07 me\r\nnext"
	got := sanitizeDescription(&dirty)
	if got == nil || *got != "clean me\nnext" {
		t.Errorf("sanitizeDescription(%q) = %v, want \"clean me\\nnext\"", dirty, got)
	}
}

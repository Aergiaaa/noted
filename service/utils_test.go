package service

import (
	"math"
	"testing"
	"time"
)

func TestParseUUID(t *testing.T) {
	clean, err := parseUUID("0197f1a0-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !clean.Valid {
		t.Fatalf("expected valid uuid")
	}

	for _, bad := range []string{"", "nope", "0197f1a0-0000"} {
		if _, err := parseUUID(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestParseDateScans(t *testing.T) {
	clean, err := parseDate(time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !clean.Valid {
		t.Fatalf("expected valid date")
	}
}

func TestParseNum(t *testing.T) {
	for name, tc := range map[string]struct {
		in   float64
		want float64
	}{
		"integer":  {in: 5000000, want: 5000000},
		"decimal":  {in: 25000.5, want: 25000.5},
		"small":    {in: 0.75, want: 0.75},
		"negative": {in: -100, want: -100},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := parseNum(tc.in)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			got, err := res.Float64Value()
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got.Float64 != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got.Float64)
			}
		})
	}
}

func TestParseNumFloat64Regression(t *testing.T) {
	res, err := parseNum(25000.5)
	if err != nil {
		t.Fatalf("float64 amount must scan: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid numeric")
	}
}

func TestParseNumInt(t *testing.T) {
	res, err := parseNum(42)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, err := res.Float64Value()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Float64 != 42 {
		t.Fatalf("expected 42, got %v", got.Float64)
	}
}

func TestParseNumRejectsNonFinite(t *testing.T) {
	for name, in := range map[string]float64{
		"inf":  math.Inf(1),
		"-inf": math.Inf(-1),
		"nan":  math.NaN(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseNum(in); err == nil {
				t.Fatalf("expected error for %v", in)
			}
		})
	}
}

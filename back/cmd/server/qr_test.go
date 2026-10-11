package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureOutput runs fn with os.Stdout/os.Stderr swapped for temp files
// and returns both captured streams.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outF, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errF, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outF, errF
	fn()
	os.Stdout, os.Stderr = oldOut, oldErr
	_ = outF.Close()
	_ = errF.Close()

	ob, err := os.ReadFile(outF.Name())
	if err != nil {
		t.Fatal(err)
	}
	eb, err := os.ReadFile(errF.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(ob), string(eb)
}

func TestRenderQR_rendersEveryHalfBlockCase(t *testing.T) {
	// 3×3 scripted matrix: pairs (top,bottom) = (1,1), (1,0), (0,1) on the
	// first text row, then a lone light bottom row — all four branches.
	dark := map[[2]int]bool{
		{0, 0}: true, {0, 1}: true, // █
		{1, 0}: true, {1, 1}: false, // ▀
		{2, 0}: false, {2, 1}: true, // ▄
		{0, 2}: false, {1, 2}: false, {2, 2}: false, // trailing spaces
	}
	out, errOut := captureOutput(t, func() {
		renderQR(3, func(x, y int) bool { return dark[[2]int{x, y}] })
	})

	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2 (two matrix rows per text row)", len(lines))
	}
	if lines[0] != "█▀▄" {
		t.Fatalf("row 0 = %q, want %q", lines[0], "█▀▄")
	}
	if lines[1] != "   " {
		t.Fatalf("row 1 = %q, want three spaces", lines[1])
	}
}

func TestPrintQR_overlongInput_reportsEncodeFailure(t *testing.T) {
	// The otpauth URI always fits; an absurd string exercises the error
	// path without panicking or writing a broken matrix.
	_, errOut := captureOutput(t, func() {
		printQR(strings.Repeat("x", 1<<20))
	})
	if !strings.Contains(errOut, "qr encode") {
		t.Fatalf("stderr = %q, want qr encode failure", errOut)
	}
}

func TestPrintQR_validURI_rendersMatrix(t *testing.T) {
	out, errOut := captureOutput(t, func() {
		printQR("otpauth://totp/Noted:me?issuer=Noted&secret=JBSWY3DPEHPK3PXP")
	})
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}
	if !strings.Contains(out, "█") {
		t.Fatalf("stdout has no dark modules: %q", out)
	}
}

func TestWriteQRSVG_writesScannableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qr.svg")
	if err := writeQRSVG("otpauth://totp/Noted:me?secret=JBSWY3DPEHPK3PXP", path); err != nil {
		t.Fatalf("writeQRSVG: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "<svg") {
		t.Fatalf("file does not contain <svg: %.80s", b)
	}
}

func TestWriteQRSVG_overlongInput_returnsEncodeError(t *testing.T) {
	err := writeQRSVG(strings.Repeat("x", 1<<20), filepath.Join(t.TempDir(), "qr.svg"))
	if err == nil {
		t.Fatal("want encode error")
	}
}

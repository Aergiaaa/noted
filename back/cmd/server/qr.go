package main

import (
	"fmt"
	"os"

	qrcode "github.com/piglig/go-qr"
)

// QR_SCALE/QR_BORDER size the optional --qr SVG: 8px per module with a
// 4-module quiet zone, comfortably scannable from a screen.
const (
	QR_SCALE  = 8
	QR_BORDER = 4
)

// printQR renders the otpauth URI as an ASCII QR on stdout: enrollment is
// a terminal flow, so no image viewer is required. Encode failures (only
// absurd input — the URI always fits) are reported, not fatal: the URI is
// printed anyway as a copy-paste fallback.
func printQR(uri string) {
	code, err := qrcode.EncodeText(uri, qrcode.Medium)
	if err != nil {
		fmt.Fprintln(os.Stderr, "qr encode:", err)
		return
	}
	renderQR(code.Size(), code.Module)
}

// renderQR prints the matrix with half-block characters: terminal cells
// are roughly 2:1, so two matrix rows map to one text row (▀ ▄ █ or a
// space for a light pair).
func renderQR(size int, module func(x, y int) bool) {
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x++ {
			top := module(x, y)
			bottom := y+1 < size && module(x, y+1)
			switch {
			case top && bottom:
				fmt.Print("█")
			case top:
				fmt.Print("▀")
			case bottom:
				fmt.Print("▄")
			default:
				fmt.Print(" ")
			}
		}
		fmt.Println()
	}
}

// writeQRSVG saves the same QR as a standalone SVG file (DEPLOYMENT.md
// suggests scanning from a second device).
func writeQRSVG(uri, path string) error {
	code, err := qrcode.EncodeText(uri, qrcode.Medium)
	if err != nil {
		return err
	}
	return code.SVG(qrcode.NewQrCodeImgConfig(QR_SCALE, QR_BORDER), path)
}

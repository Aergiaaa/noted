package main

import (
	"fmt"
	"os"

	qrcode "github.com/piglig/go-qr"
)

const (
	QR_SCALE  = 8
	QR_BORDER = 4
)

// printQR: an encode failure is not fatal — the URI is printed anyway as
// a copy-paste fallback.
func printQR(uri string) {
	code, err := qrcode.EncodeText(uri, qrcode.Medium)
	if err != nil {
		fmt.Fprintln(os.Stderr, "qr encode:", err)
		return
	}
	renderQR(code.Size(), code.Module)
}

// renderQR uses half-block characters because terminal cells are roughly
// 2:1 — two matrix rows fit in one text row.
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

func writeQRSVG(uri, path string) error {
	code, err := qrcode.EncodeText(uri, qrcode.Medium)
	if err != nil {
		return err
	}
	return code.SVG(qrcode.NewQrCodeImgConfig(QR_SCALE, QR_BORDER), path)
}

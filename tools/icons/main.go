// Command icons draws Haepage's app icons (the "H" logo from the favicon)
// as PNGs for the web app manifest. Run from the repo root:
//
//	go run ./tools/icons web/public/icons
//
// The output is committed; rerun only when the logo changes.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

var (
	purple = color.RGBA{0x67, 0x50, 0xa4, 0xff}
	white  = color.RGBA{0xff, 0xff, 0xff, 0xff}
)

// inside reports whether (x, y) on the favicon's 32-unit grid is covered:
// a rounded square (radius 8), and the H (M9 8h4v6h6V8h4v16h-4v-6h-6v6H9z).
// full drops the rounded corners, for maskable icons that the system crops.
func inside(x, y float64, full bool) (bg, fg bool) {
	bg = full || roundedRect(x, y, 32, 8)
	fg = bg && y >= 8 && y <= 24 && ((x >= 9 && x <= 13) || (x >= 19 && x <= 23) || (x > 13 && x < 19 && y >= 14 && y <= 18))
	return bg, fg
}

func roundedRect(x, y, size, r float64) bool {
	cx := min(max(x, r), size-r)
	cy := min(max(y, r), size-r)
	return x >= 0 && y >= 0 && x <= size && y <= size && (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
}

// draw renders the logo at px pixels with 4×4 supersampling. Maskable icons
// shrink the H into the central safe zone (80 %).
func draw(px int, maskable bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, px, px))
	const ss = 4
	scale := 32 / float64(px)
	for y := 0; y < px; y++ {
		for x := 0; x < px; x++ {
			var bgN, fgN int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					gx := (float64(x) + (float64(sx)+0.5)/ss) * scale
					gy := (float64(y) + (float64(sy)+0.5)/ss) * scale
					if maskable { // logo at 70 %, centered
						gx, gy = (gx-16)/0.7+16, (gy-16)/0.7+16
					}
					bg, fg := inside(gx, gy, maskable)
					if bg {
						bgN++
					}
					if fg {
						fgN++
					}
				}
			}
			const n = ss * ss
			if maskable {
				bgN = n
			}
			// Blend white over purple by fg coverage; alpha from bg coverage.
			mix := func(a, b uint8) uint8 {
				return uint8((int(a)*(bgN-fgN) + int(b)*fgN) / max(bgN, 1))
			}
			img.SetRGBA(x, y, color.RGBA{mix(purple.R, white.R), mix(purple.G, white.G), mix(purple.B, white.B), uint8(255 * bgN / n)})
		}
	}
	return premultiply(img)
}

// image.RGBA is premultiplied; scale colors by alpha at the soft edges.
func premultiply(img *image.RGBA) *image.RGBA {
	for i := 0; i < len(img.Pix); i += 4 {
		a := int(img.Pix[i+3])
		for c := 0; c < 3; c++ {
			img.Pix[i+c] = uint8(int(img.Pix[i+c]) * a / 255)
		}
	}
	return img
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/icons <out dir>")
		os.Exit(2)
	}
	out := os.Args[1]
	icons := []struct {
		name     string
		px       int
		maskable bool
	}{
		{"icon-192.png", 192, false},
		{"icon-512.png", 512, false},
		{"maskable-512.png", 512, true},
		{"apple-touch-icon.png", 180, true}, // iOS rounds the corners itself
	}
	for _, ic := range icons {
		f, err := os.Create(filepath.Join(out, ic.name))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(f, draw(ic.px, ic.maskable)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		f.Close()
	}
}

// Package wallpaper turns the images in the user's wallpaper folder into
// sized JPEG copies and a tiny pre-blurred copy for frosted-glass themes
// (IMPLEMENTATION.md M3.6). Only the standard library is used: JPEG, PNG
// and GIF are decoded and JPEG is written. WebP and AVIF are served as they
// are, without a blurred copy.
package wallpaper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // register decoders
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/haelahpx/haepage/internal/config"
)

// Widths of the sized copies, and the blurred copy's width.
var Widths = []int{1280, 1920, 2560}

const (
	BlurWidth  = 64
	Budget     = 500 << 10 // bytes per copy (IMPLEMENTATION.md §9)
	maxPixels  = 40_000_000
	maxFileLen = 50 << 20
)

// ErrNotFound means there is no such wallpaper or copy.
var ErrNotFound = errors.New("wallpaper not found")

var decodable = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true}
var passThrough = map[string]string{".webp": "image/webp", ".avif": "image/avif"}

// Info describes one wallpaper. Hash identifies its content, so copy URLs
// can be cached forever.
type Info struct {
	Name    string `json:"name"`
	Hash    string `json:"hash"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Sizes   []int  `json:"sizes"`   // widths available, smallest first
	Blurred bool   `json:"blurred"` // a pre-blurred copy exists
}

// Library reads wallpapers from Dir and keeps processed copies in Cache.
type Library struct {
	Dir, Cache string

	mu    sync.Mutex
	known map[string]Info // by hash
}

// List returns the wallpapers in the folder, processing new ones. Copies of
// wallpapers that are gone are deleted.
func (l *Library) List() ([]Info, error) {
	entries, err := os.ReadDir(l.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Info{}, nil
	}
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	known := map[string]Info{}
	list := []Info{}
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || (!decodable[ext] && passThrough[ext] == "") {
			continue
		}
		info, err := l.process(e.Name())
		if err != nil {
			continue // unreadable or too large: leave it out
		}
		known[info.Hash] = info
		list = append(list, info)
	}
	l.known = known
	l.prune()
	return list, nil
}

// File returns a copy: variant is a width from Sizes or "blur". It returns
// the path to serve and its content type.
func (l *Library) File(hash, variant string) (string, string, error) {
	l.mu.Lock()
	info, ok := l.known[hash]
	l.mu.Unlock()
	if !ok {
		if _, err := l.List(); err != nil {
			return "", "", err
		}
		l.mu.Lock()
		info, ok = l.known[hash]
		l.mu.Unlock()
		if !ok {
			return "", "", ErrNotFound
		}
	}
	if ctype := passThrough[strings.ToLower(filepath.Ext(info.Name))]; ctype != "" {
		if variant == "blur" {
			return "", "", ErrNotFound
		}
		return filepath.Join(l.Dir, info.Name), ctype, nil
	}
	if variant == "blur" && !info.Blurred {
		return "", "", ErrNotFound
	}
	if variant != "blur" && !slices.ContainsFunc(info.Sizes, func(w int) bool { return fmt.Sprint(w) == variant }) {
		return "", "", ErrNotFound
	}
	return l.copyPath(hash, variant), "image/jpeg", nil
}

func (l *Library) copyPath(hash, variant string) string {
	return filepath.Join(l.Cache, hash+"-"+variant+".jpg")
}

// process makes the copies for one file unless they exist already.
func (l *Library) process(name string) (Info, error) {
	f, err := os.Open(filepath.Join(l.Dir, name))
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileLen+1))
	if err != nil {
		return Info{}, err
	}
	if len(data) > maxFileLen {
		return Info{}, fmt.Errorf("%s is larger than %d MB", name, maxFileLen>>20)
	}
	sum := sha256.Sum256(data)
	info := Info{Name: name, Hash: hex.EncodeToString(sum[:8])}

	if passThrough[strings.ToLower(filepath.Ext(name))] != "" {
		info.Sizes = []int{0} // the original, at its own size
		return info, nil
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Info{}, err
	}
	if cfg.Width*cfg.Height > maxPixels {
		return Info{}, fmt.Errorf("%s has more than %d megapixels", name, maxPixels/1_000_000)
	}
	info.Width, info.Height = cfg.Width, cfg.Height
	// Every standard width below the original, plus the original's own
	// width as the largest copy when it is smaller than the biggest.
	for _, w := range Widths {
		if w < cfg.Width {
			info.Sizes = append(info.Sizes, w)
		}
	}
	if cfg.Width < Widths[len(Widths)-1] {
		info.Sizes = append(info.Sizes, cfg.Width)
	}
	info.Blurred = true

	if l.haveCopies(info) {
		return info, nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Info{}, err
	}
	img := toRGBA(decoded)
	if err := os.MkdirAll(l.Cache, 0o755); err != nil {
		return Info{}, err
	}
	for _, w := range info.Sizes {
		if err := writeJPEG(l.copyPath(info.Hash, fmt.Sprint(w)), Resize(img, w), 82); err != nil {
			return Info{}, err
		}
	}
	if err := writeJPEG(l.copyPath(info.Hash, "blur"), Blur(Resize(img, BlurWidth), 3), 70); err != nil {
		return Info{}, err
	}
	return info, nil
}

func (l *Library) haveCopies(info Info) bool {
	variants := []string{"blur"}
	for _, w := range info.Sizes {
		variants = append(variants, fmt.Sprint(w))
	}
	for _, v := range variants {
		if _, err := os.Stat(l.copyPath(info.Hash, v)); err != nil {
			return false
		}
	}
	return true
}

// prune deletes copies whose wallpaper is no longer in the folder.
func (l *Library) prune() {
	entries, _ := os.ReadDir(l.Cache)
	for _, e := range entries {
		hash, _, ok := strings.Cut(e.Name(), "-")
		if _, keep := l.known[hash]; ok && !keep {
			os.Remove(filepath.Join(l.Cache, e.Name()))
		}
	}
}

// writeJPEG encodes img, lowering the quality until it fits the budget.
func writeJPEG(path string, img image.Image, quality int) error {
	var buf bytes.Buffer
	for q := quality; ; q -= 10 {
		buf.Reset()
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return err
		}
		if buf.Len() <= Budget || q <= 40 {
			break
		}
	}
	return config.WriteFileAtomic(path, buf.Bytes(), 0o644)
}

func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

// Resize scales src to width (keeping its shape) by averaging the source
// pixels each output pixel covers, which is sharp and alias-free when
// shrinking. Images narrower than width are returned as they are.
func Resize(src *image.RGBA, width int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if width >= sw {
		return src
	}
	height := max(1, (sh*width+sw/2)/sw)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		y0, y1 := y*sh/height, max((y+1)*sh/height, y*sh/height+1)
		for x := 0; x < width; x++ {
			x0, x1 := x*sw/width, max((x+1)*sw/width, x*sw/width+1)
			var r, g, bl, a, n int
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4 : sx*4+4]
					r, g, bl, a = r+int(p[0]), g+int(p[1]), bl+int(p[2]), a+int(p[3])
					n++
				}
			}
			o := dst.Pix[y*dst.Stride+x*4:]
			o[0], o[1], o[2], o[3] = uint8(r/n), uint8(g/n), uint8(bl/n), uint8(a/n)
		}
	}
	return dst
}

// Blur runs a 3-pass box blur of the given radius, which approximates a
// gaussian. On a 64 px copy that is stretched to the screen, it looks the
// same as a large live blur at none of the cost.
func Blur(img *image.RGBA, radius int) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	tmp := image.NewRGBA(img.Bounds())
	for pass := 0; pass < 3; pass++ {
		boxPass(out, tmp, radius, true)
		boxPass(tmp, out, radius, false)
	}
	return out
}

func boxPass(src, dst *image.RGBA, radius int, horizontal bool) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	lines, length := h, w
	if !horizontal {
		lines, length = w, h
	}
	at := func(line, i int) int {
		i = min(max(i, 0), length-1) // clamp at the edges
		if horizontal {
			return line*src.Stride + i*4
		}
		return i*src.Stride + line*4
	}
	span := 2*radius + 1
	for line := 0; line < lines; line++ {
		for i := 0; i < length; i++ {
			var sum [4]int
			for k := -radius; k <= radius; k++ {
				p := at(line, i+k)
				for c := 0; c < 4; c++ {
					sum[c] += int(src.Pix[p+c])
				}
			}
			o := at(line, i)
			for c := 0; c < 4; c++ {
				dst.Pix[o+c] = uint8(sum[c] / span)
			}
		}
	}
}

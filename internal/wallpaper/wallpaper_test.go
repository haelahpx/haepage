package wallpaper

import (
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeImage(t *testing.T, path string, w, h int, encode func(*os.File, image.Image) error) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Smooth gradients with some texture, like a photo.
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), uint8((x/9 + y/7) % 64), 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func encodePNG(f *os.File, img image.Image) error { return png.Encode(f, img) }
func encodeJPEG(f *os.File, img image.Image) error {
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
}

func TestLibrary(t *testing.T) {
	dir := t.TempDir()
	lib := &Library{Dir: filepath.Join(dir, "wallpapers"), Cache: filepath.Join(dir, "cache")}

	if list, err := lib.List(); err != nil || len(list) != 0 {
		t.Fatalf("missing folder: %v, %v", list, err)
	}
	os.MkdirAll(lib.Dir, 0o755)
	writeImage(t, filepath.Join(lib.Dir, "big.jpg"), 3000, 2000, encodeJPEG)
	writeImage(t, filepath.Join(lib.Dir, "small.png"), 1500, 1000, encodePNG)
	os.WriteFile(filepath.Join(lib.Dir, "photo.webp"), []byte("RIFF0000WEBP"), 0o644)
	os.WriteFile(filepath.Join(lib.Dir, "notes.txt"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(lib.Dir, "broken.jpg"), []byte("not a jpeg"), 0o644)

	list, err := lib.List()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Info{}
	for _, i := range list {
		byName[i.Name] = i
	}
	if len(byName) != 3 {
		t.Fatalf("listed %v, want big.jpg, small.png and photo.webp", list)
	}
	if got := byName["big.jpg"].Sizes; len(got) != 3 || got[0] != 1280 || got[2] != 2560 {
		t.Errorf("big.jpg sizes = %v", got)
	}
	if got := byName["small.png"].Sizes; len(got) != 2 || got[0] != 1280 || got[1] != 1500 {
		t.Errorf("small.png sizes = %v", got)
	}
	if w := byName["photo.webp"]; w.Blurred || len(w.Sizes) != 1 {
		t.Errorf("webp = %+v, want the original only", w)
	}

	big := byName["big.jpg"]
	for _, v := range []string{"1280", "1920", "2560", "blur"} {
		path, ctype, err := lib.File(big.Hash, v)
		if err != nil || ctype != "image/jpeg" {
			t.Fatalf("File(%s) = %v %v", v, ctype, err)
		}
		st, _ := os.Stat(path)
		if st.Size() > Budget {
			t.Errorf("%s is %d bytes, over the budget", v, st.Size())
		}
		f, _ := os.Open(path)
		cfg, err := jpeg.DecodeConfig(f)
		f.Close()
		want := map[string]int{"1280": 1280, "1920": 1920, "2560": 2560, "blur": BlurWidth}[v]
		if wantH := (want*2000 + 1500) / 3000; err != nil || cfg.Width != want || cfg.Height != wantH {
			t.Errorf("%s is %dx%d (%v), want width %d", v, cfg.Width, cfg.Height, err, want)
		}
	}
	if _, _, err := lib.File(big.Hash, "800"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown size: %v", err)
	}
	if _, _, err := lib.File("nope", "1280"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown hash: %v", err)
	}
	webp := byName["photo.webp"]
	if path, ctype, err := lib.File(webp.Hash, "0"); err != nil || ctype != "image/webp" || filepath.Base(path) != "photo.webp" {
		t.Errorf("webp original = %s %s %v", path, ctype, err)
	}
	if _, _, err := lib.File(webp.Hash, "blur"); !errors.Is(err, ErrNotFound) {
		t.Errorf("webp blur: %v", err)
	}

	// Removing a wallpaper deletes its copies on the next listing.
	os.Remove(filepath.Join(lib.Dir, "big.jpg"))
	lib.List()
	if matches, _ := filepath.Glob(filepath.Join(lib.Cache, big.Hash+"-*")); len(matches) != 0 {
		t.Errorf("stale copies left: %v", matches)
	}
}

func TestResizeAveragesPixels(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := 0; x < 4; x++ {
		c := uint8(0)
		if x%2 == 1 {
			c = 200
		}
		src.Set(x, 0, color.RGBA{c, c, c, 255})
		src.Set(x, 1, color.RGBA{c, c, c, 255})
	}
	out := Resize(src, 2)
	if b := out.Bounds(); b.Dx() != 2 || b.Dy() != 1 {
		t.Fatalf("size %v", b)
	}
	if r := out.RGBAAt(0, 0).R; r != 100 {
		t.Errorf("averaged pixel = %d, want 100", r)
	}
}

func TestBlurSpreadsAPoint(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 21, 21))
	img.Set(10, 10, color.RGBA{255, 255, 255, 255})
	out := Blur(img, 2)
	center, near, far := out.RGBAAt(10, 10).R, out.RGBAAt(13, 10).R, out.RGBAAt(0, 0).R
	if !(center > near && near > 0 && far == 0) {
		t.Errorf("center %d, near %d, far %d", center, near, far)
	}
}

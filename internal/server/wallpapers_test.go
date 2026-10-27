package server

import (
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haelahpx/haepage/internal/wallpaper"
)

func TestWallpapers(t *testing.T) {
	dir := t.TempDir()
	lib := &wallpaper.Library{Dir: filepath.Join(dir, "w"), Cache: filepath.Join(dir, "c")}
	os.MkdirAll(lib.Dir, 0o755)
	f, _ := os.Create(filepath.Join(lib.Dir, "sky.png"))
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 1400, 900)))
	f.Close()
	h := New(Options{Static: built(), Wallpapers: lib})

	if !strings.Contains(get(t, h, http.MethodGet, "/api/ping").Body.String(), `"wallpapers"`) {
		t.Error("ping lacks wallpapers")
	}
	rec := get(t, h, http.MethodGet, "/api/wallpapers")
	var list []wallpaper.Info
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("list = %s", rec.Body)
	}
	sky := list[0]
	if sky.Name != "sky.png" || len(sky.Sizes) != 2 || !sky.Blurred {
		t.Fatalf("sky = %+v", sky)
	}

	rec = get(t, h, http.MethodGet, "/wallpapers/"+sky.Hash+"/blur")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("blur = %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	for _, path := range []string{"/wallpapers/" + sky.Hash + "/999", "/wallpapers/nope/1280", "/wallpapers/" + sky.Hash + "/..%2f..%2fsecrets.json"} {
		if rec := get(t, h, http.MethodGet, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, rec.Code)
		}
	}
}

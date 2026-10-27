package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/haelahpx/haepage/internal/wallpaper"
)

func listWallpapers(lib *wallpaper.Library) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := lib.List()
		if err != nil {
			slog.Error("listing wallpapers", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read the wallpaper folder"})
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// wallpaperFile serves a processed copy. URLs carry the content hash, so
// they can be cached for good.
func wallpaperFile(lib *wallpaper.Library) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path, ctype, err := lib.File(r.PathValue("hash"), r.PathValue("variant"))
		if errors.Is(err, wallpaper.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			slog.Error("serving wallpaper", "err", err)
			http.Error(w, "could not read the wallpaper", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Content-Type", ctype)
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
		http.ServeFile(w, r, path)
	}
}

package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/haelahpx/haepage/internal/proxy"
	"github.com/haelahpx/haepage/internal/themes"
)

// importTheme fetches a theme link through the SSRF guard and keeps a
// pinned copy. {refresh: true} fetches it again.
func importTheme(remote *themes.Remote) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL     string `json:"url"`
			Refresh bool   `json:"refresh"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&body); err != nil || body.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected {url}"})
			return
		}
		info, err := remote.Import(r.Context(), body.URL, body.Refresh)
		switch {
		case errors.Is(err, themes.ErrInvalid), errors.Is(err, proxy.ErrBlocked):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case err != nil:
			slog.Info("theme import failed", "url", body.URL, "err", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusOK, info)
		}
	}
}

// remoteTheme serves the pinned copy of an imported theme.
func remoteTheme(remote *themes.Remote) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		css, info, err := remote.Get(r.URL.Query().Get("url"))
		if errors.Is(err, themes.ErrNotImported) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "theme not imported"})
			return
		}
		if err != nil {
			slog.Error("reading cached theme", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read the theme"})
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/css; charset=utf-8")
		h.Set("Cache-Control", "no-cache")
		h.Set("ETag", `"`+info.SHA256+`"`)
		h.Set("X-Theme-Sha256", info.SHA256)
		if r.Header.Get("If-None-Match") == `"`+info.SHA256+`"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Write(css)
	}
}

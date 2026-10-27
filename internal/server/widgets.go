package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/proxy"
)

// widgetData answers an API widget's request through the server, so its
// secrets stay here.
func widgetData(w *proxy.Widgets) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		body, err := w.Data(r.Context(), r.PathValue("id"))
		switch {
		case errors.Is(err, proxy.ErrNoWidget):
			writeJSON(rw, http.StatusNotFound, map[string]string{"error": err.Error()})
		case errors.Is(err, proxy.ErrWidget), errors.Is(err, proxy.ErrBlocked):
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case err != nil:
			slog.Info("widget fetch failed", "widget", r.PathValue("id"), "err", err.Error())
			writeJSON(rw, http.StatusBadGateway, map[string]string{"error": err.Error()})
		default:
			rw.Header().Set("Content-Type", "application/json")
			rw.Header().Set("Cache-Control", "no-store")
			rw.Write(body)
		}
	}
}

// listSecrets returns the names of widget secrets, never their values.
// Integration keys and webhook tokens (names with ':') are not listed.
func listSecrets(s *config.Secrets) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names := []string{}
		for _, n := range s.Names() {
			if proxy.SecretName.MatchString(n) {
				names = append(names, n)
			}
		}
		slices.Sort(names)
		writeJSON(w, http.StatusOK, map[string][]string{"names": names})
	}
}

// putSecret sets a widget secret: PUT /api/secrets/{name} {"value": "…"}.
// An empty value deletes it.
func putSecret(s *config.Secrets) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !proxy.SecretName.MatchString(name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "use lowercase letters, digits, - and _ for secret names"})
			return
		}
		var body struct {
			Value string `json:"value"`
		}
		if r.Method == http.MethodPut {
			if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected {value}"})
				return
			}
		}
		if err := s.Set(name, strings.TrimSpace(body.Value)); err != nil {
			slog.Error("saving secret", "name", name, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save the secret"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

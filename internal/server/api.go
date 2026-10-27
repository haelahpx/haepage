package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/integrations"
	"github.com/haelahpx/haepage/internal/store"
)

// heartbeat keeps idle SSE connections open through proxies.
var heartbeat = 25 * time.Second

func getConfig(cfg *config.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, etag := cfg.Get()
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
	}
}

// putConfig replaces haepage.json. The client must send the ETag it read
// (If-Match), so a save from a stale tab can't overwrite newer changes.
func putConfig(cfg *config.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ifMatch := r.Header.Get("If-Match")
		if ifMatch == "" {
			writeJSON(w, http.StatusPreconditionRequired, map[string]string{"error": "If-Match header required"})
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, config.MaxSize+1))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not read body"})
			return
		}
		etag, err := cfg.Put(raw, ifMatch)
		switch {
		case errors.Is(err, config.ErrConflict):
			_, current := cfg.Get()
			w.Header().Set("ETag", current)
			writeJSON(w, http.StatusPreconditionFailed, map[string]string{"error": err.Error()})
		case errors.Is(err, config.ErrInvalid):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case err != nil:
			slog.Error("saving config", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save config"})
		default:
			w.Header().Set("ETag", etag)
			writeJSON(w, http.StatusOK, map[string]string{"etag": etag})
		}
	}
}

func listServices(reg *store.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, reg.Snapshot())
	}
}

// putOverride saves the user's choice for one detected service: pin it
// (with a group), hide it, rename it. A body of null removes the override.
// The service id goes last in the path because ids contain slashes.
func putOverride(cfg *config.Store, reg *store.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := reg.Get(id); !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown service"})
			return
		}
		var o *config.Override
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&o); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be an override object or null"})
			return
		}
		etag, err := cfg.SetOverride(id, o)
		if err != nil {
			slog.Error("saving override", "id", id, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save override"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"etag": etag})
	}
}

// events streams service, health and config changes as server-sent events.
// Each connection starts with a full snapshot, so a client that
// reconnects is always up to date.
func events(reg *store.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Accel-Buffering", "no") // don't buffer behind nginx-style proxies

		ch, stop := reg.Subscribe() // before the snapshot, so nothing is missed
		defer stop()
		send := func(event string, data any) error {
			b, err := json.Marshal(data)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		}
		if send("snapshot", reg.State()) != nil {
			return
		}

		beat := time.NewTicker(heartbeat)
		defer beat.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-beat.C:
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case e, open := <-ch:
				if !open {
					return // fell behind; the browser reconnects and gets a snapshot
				}
				if send(e.Type, e.Data) != nil {
					return
				}
			}
		}
	}
}

func listIntegrations(m *integrations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := m.List()
		if list == nil {
			list = []integrations.Info{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// connectIntegration tests an API key against the app and saves it.
func connectIntegration(m *integrations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Service string `json:"service"`
			Key     string `json:"key"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected {service, key}"})
			return
		}
		err := m.Connect(r.Context(), body.Service, body.Key)
		switch {
		case errors.Is(err, integrations.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		case err != nil:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusOK, map[string]bool{"connected": true})
		}
	}
}

func disconnectIntegration(m *integrations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Service string `json:"service"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&body); err != nil || body.Service == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected {service}"})
			return
		}
		if err := m.Disconnect(body.Service); err != nil {
			slog.Error("disconnecting integration", "service", body.Service, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not disconnect"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"connected": false})
	}
}

// integrationImage proxies a poster that the app only serves with its API
// key. Only images pass, and they are sandboxed: an SVG can carry script.
func integrationImage(m *integrations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, ctype, err := m.Image(r.Context(), r.URL.Query().Get("service"), r.URL.Query().Get("item"))
		if err != nil || !strings.HasPrefix(ctype, "image/") {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", ctype)
		h.Set("Cache-Control", "private, max-age=86400")
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		w.Write(data)
	}
}

// webhook receives notifications from connected apps. The random token in
// the path identifies (and authorizes) the integration.
func webhook(m *integrations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 256<<10))
		if err != nil || !m.Webhook(r.PathValue("token"), body) {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/haelahpx/haepage/internal/config"
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
		if send("snapshot", map[string]any{"services": reg.Snapshot()}) != nil {
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

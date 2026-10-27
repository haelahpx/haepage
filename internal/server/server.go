// Package server wires Haepage's HTTP routes: the API and the frontend files.
package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/integrations"
	"github.com/haelahpx/haepage/internal/store"
	"github.com/haelahpx/haepage/internal/system"
)

// APIVersion is the version of the /api contract. It changes only on a
// breaking change.
const APIVersion = 1

// Options configures the HTTP handler.
type Options struct {
	Version      string
	Static       fs.FS                 // frontend build output, rooted at the folder holding index.html
	Dev          bool                  // turn off browser caching
	System       SystemReader          // host stats for /api/system; nil turns the endpoint off
	Config       *config.Store         // haepage.json; nil turns the config API off
	Registry     *store.Registry       // merged services; nil turns services and events off
	Detect       bool                  // discovery is running
	Integrations *integrations.Manager // nil turns integrations off
	Health       bool                  // the server runs health checks
}

// SystemReader provides host stats.
type SystemReader interface {
	Read() (system.Stats, error)
}

// PingResponse is the body of GET /api/ping. The frontend turns features on
// only from Capabilities.
type PingResponse struct {
	App          string   `json:"app"`
	Version      string   `json:"version"`
	API          int      `json:"api"`
	Auth         PingAuth `json:"auth"`
	Capabilities []string `json:"capabilities"`
}

// PingAuth describes the login state of the caller.
type PingAuth struct {
	Enabled bool    `json:"enabled"`
	User    *string `json:"user"`
	Role    string  `json:"role"`
}

// New returns the handler for every Haepage route.
func New(opts Options) http.Handler {
	capabilities := []string{}
	mux := http.NewServeMux()
	if opts.System != nil {
		capabilities = append(capabilities, "system")
		mux.HandleFunc("GET /api/system", systemStats(opts.System))
	}
	if opts.Config != nil {
		capabilities = append(capabilities, "config.write")
		mux.HandleFunc("GET /haepage.json", getConfig(opts.Config))
		mux.HandleFunc("GET /api/config", getConfig(opts.Config))
		mux.HandleFunc("PUT /api/config", putConfig(opts.Config))
	}
	if opts.Registry != nil {
		capabilities = append(capabilities, "events")
		mux.HandleFunc("GET /api/services", listServices(opts.Registry))
		mux.HandleFunc("GET /api/events", events(opts.Registry))
		if opts.Config != nil {
			mux.HandleFunc("PUT /api/overrides/{id...}", putOverride(opts.Config, opts.Registry))
		}
	}
	if opts.Integrations != nil {
		capabilities = append(capabilities, "integrations")
		mux.HandleFunc("GET /api/integrations", listIntegrations(opts.Integrations))
		mux.HandleFunc("POST /api/integrations/connect", connectIntegration(opts.Integrations))
		mux.HandleFunc("POST /api/integrations/disconnect", disconnectIntegration(opts.Integrations))
		mux.HandleFunc("GET /api/integrations/image", integrationImage(opts.Integrations))
		mux.HandleFunc("POST /api/hooks/{token}", webhook(opts.Integrations))
	}
	if opts.Detect {
		capabilities = append(capabilities, "services.detect")
	}
	if opts.Health {
		capabilities = append(capabilities, "health.full")
	}
	mux.HandleFunc("GET /api/ping", ping(opts.Version, capabilities))
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	mux.Handle("GET /", static(opts.Static, opts.Dev))
	return securityHeaders(mux)
}

func ping(version string, capabilities []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, PingResponse{
			App:          "haepage",
			Version:      version,
			API:          APIVersion,
			Auth:         PingAuth{Enabled: false, Role: "admin"},
			Capabilities: capabilities,
		})
	}
}

func systemStats(reader SystemReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := reader.Read()
		if err != nil {
			slog.Warn("reading system stats", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "system stats unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

func static(fsys fs.FS, dev bool) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Checked per request so --dev picks up a build made after startup.
		if _, err := fs.Stat(fsys, "index.html"); err != nil {
			notBuilt(w)
			return
		}
		if dev || r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func notBuilt(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>Haepage</title>` +
		`<h1>Haepage</h1><p>The frontend is not built yet. Run <code>make web</code>, then restart the server.</p>` +
		`<p>The API is up: <a href="/api/ping">/api/ping</a>.</p>`))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

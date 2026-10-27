package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/haelahpx/haepage/internal/history"
	"github.com/haelahpx/haepage/internal/system"
)

func get(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func built() fstest.MapFS {
	return fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><title>hp</title>")},
		"app.js":     {Data: []byte("console.log(1)")},
		"sw.js":      {Data: []byte("self.addEventListener('fetch', () => {})")},

		"manifest.webmanifest": {Data: []byte(`{"name":"Haepage"}`)},
	}
}

func TestPing(t *testing.T) {
	h := New(Options{Version: "1.2.3", Static: built()})
	rec := get(t, h, http.MethodGet, "/api/ping")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	// With no optional features on, only the always-available lint is listed.
	if !strings.Contains(rec.Body.String(), `"capabilities":["themes.lint"]`) {
		t.Errorf("capabilities = %s, want only themes.lint", rec.Body)
	}
	var got PingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.App != "haepage" || got.Version != "1.2.3" || got.API != APIVersion {
		t.Errorf("ping = %+v", got)
	}
	if got.Auth.Enabled || got.Auth.User != nil || got.Auth.Role != "admin" {
		t.Errorf("auth = %+v", got.Auth)
	}
}

func TestPingWrongMethod(t *testing.T) {
	rec := get(t, New(Options{Static: built()}), http.MethodPost, "/api/ping")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestUnknownAPIRoute(t *testing.T) {
	rec := get(t, New(Options{Static: built()}), http.MethodGet, "/api/nope")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestStaticFiles(t *testing.T) {
	h := New(Options{Static: built()})

	rec := get(t, h, http.MethodGet, "/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>hp</title>") {
		t.Fatalf("GET / = %d %q", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index Cache-Control = %q, want no-cache", cc)
	}

	rec = get(t, h, http.MethodGet, "/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /app.js = %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "" {
		t.Errorf("asset Cache-Control = %q, want none outside dev mode", cc)
	}

	// The service worker and manifest must be revalidated so updates arrive.
	for path, ctype := range map[string]string{"/sw.js": "text/javascript", "/manifest.webmanifest": "application/manifest+json"} {
		rec = get(t, h, http.MethodGet, path)
		if cc := rec.Header().Get("Cache-Control"); rec.Code != http.StatusOK || cc != "no-cache" {
			t.Errorf("%s = %d, Cache-Control %q", path, rec.Code, cc)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, ctype) {
			t.Errorf("%s Content-Type = %q, want %s", path, ct, ctype)
		}
	}
	// …without making everything else uncached.
	if cc := get(t, h, http.MethodGet, "/app.js").Header().Get("Cache-Control"); cc != "" {
		t.Errorf("asset after sw.js: Cache-Control = %q", cc)
	}
}

func TestDevModeDisablesCaching(t *testing.T) {
	rec := get(t, New(Options{Static: built(), Dev: true}), http.MethodGet, "/app.js")
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
}

func TestFrontendNotBuilt(t *testing.T) {
	rec := get(t, New(Options{Static: fstest.MapFS{".gitkeep": {}}}), http.MethodGet, "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not built") {
		t.Errorf("body = %q", rec.Body)
	}
}

type fakeSystem struct {
	stats system.Stats
	err   error
}

func (f fakeSystem) Read() (system.Stats, error) { return f.stats, f.err }

func TestSystemStats(t *testing.T) {
	h := New(Options{Static: built(), System: fakeSystem{stats: system.Stats{Hostname: "box", CPU: system.CPU{Percent: 12.5}}}})

	rec := get(t, h, http.MethodGet, "/api/ping")
	if !strings.Contains(rec.Body.String(), `"capabilities":["system","themes.lint"]`) {
		t.Errorf("ping = %s, want the system capability", rec.Body)
	}

	rec = get(t, h, http.MethodGet, "/api/system")
	var got system.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("GET /api/system = %d %s", rec.Code, rec.Body)
	}
	if got.Hostname != "box" || got.CPU.Percent != 12.5 {
		t.Errorf("stats = %+v", got)
	}
}

func TestSystemStatsError(t *testing.T) {
	h := New(Options{Static: built(), System: fakeSystem{err: errors.New("boom")}})
	rec := get(t, h, http.MethodGet, "/api/system")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("GET /api/system = %d %s, want a 500 without internal details", rec.Code, rec.Body)
	}
}

func TestSystemOff(t *testing.T) {
	rec := get(t, New(Options{Static: built()}), http.MethodGet, "/api/system")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when stats are off", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := get(t, New(Options{Static: built()}), http.MethodGet, "/")
	if v := rec.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", v)
	}
	if v := rec.Header().Get("Referrer-Policy"); v != "same-origin" {
		t.Errorf("Referrer-Policy = %q", v)
	}
}

func TestHistory(t *testing.T) {
	hist := history.Open(filepath.Join(t.TempDir(), "state.json"))
	hist.Record("manual:nas", "up")
	hist.Record("manual:nas", "down")
	h := New(Options{Static: built(), History: hist})
	if !strings.Contains(get(t, h, http.MethodGet, "/api/ping").Body.String(), `"history"`) {
		t.Error("ping lacks history")
	}
	var got struct {
		BarMinutes int
		Services   map[string]history.Summary
	}
	rec := get(t, h, http.MethodGet, "/api/history")
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.BarMinutes != 30 || *got.Services["manual:nas"].Uptime != 50 {
		t.Errorf("history = %s", rec.Body)
	}
}

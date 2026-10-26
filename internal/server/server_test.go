package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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
	// An empty list must be [] and not null, or the frontend would crash on it.
	if !strings.Contains(rec.Body.String(), `"capabilities":[]`) {
		t.Errorf("capabilities not an empty array: %s", rec.Body)
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

func TestSecurityHeaders(t *testing.T) {
	rec := get(t, New(Options{Static: built()}), http.MethodGet, "/")
	if v := rec.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", v)
	}
	if v := rec.Header().Get("Referrer-Policy"); v != "same-origin" {
		t.Errorf("Referrer-Policy = %q", v)
	}
}

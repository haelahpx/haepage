package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/store"
)

func apiServer(t *testing.T) (http.Handler, *config.Store, *store.Registry) {
	t.Helper()
	cfg, err := config.Open(filepath.Join(t.TempDir(), "haepage.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := store.NewRegistry()
	reg.Set([]store.Service{{ID: "docker:media/app", Name: "App", State: "detected", Group: "inbox"}})
	return New(Options{Static: built(), Config: cfg, Registry: reg, Detect: true, Health: true}), cfg, reg
}

func send(t *testing.T, h http.Handler, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCapabilities(t *testing.T) {
	h, _, _ := apiServer(t)
	rec := get(t, h, http.MethodGet, "/api/ping")
	for _, c := range []string{"config.write", "events", "services.detect", "health.full"} {
		if !strings.Contains(rec.Body.String(), `"`+c+`"`) {
			t.Errorf("capability %q missing: %s", c, rec.Body)
		}
	}
}

func TestConfigRoundTrip(t *testing.T) {
	h, _, _ := apiServer(t)

	rec := get(t, h, http.MethodGet, "/api/config")
	etag := rec.Header().Get("ETag")
	if rec.Code != http.StatusOK || etag == "" || rec.Body.String() != `{"version":1}` {
		t.Fatalf("GET = %d %q etag %q", rec.Code, rec.Body, etag)
	}
	if get(t, h, http.MethodGet, "/haepage.json").Body.String() != `{"version":1}` {
		t.Error("/haepage.json doesn't serve the config")
	}

	body := `{"version":1,"settings":{"title":"Lab"}}`
	if rec := send(t, h, http.MethodPut, "/api/config", body, nil); rec.Code != http.StatusPreconditionRequired {
		t.Errorf("PUT without If-Match = %d, want 428", rec.Code)
	}
	rec = send(t, h, http.MethodPut, "/api/config", body, map[string]string{"If-Match": etag})
	newTag := rec.Header().Get("ETag")
	if rec.Code != http.StatusOK || newTag == "" || newTag == etag {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	// A second tab still holding the old ETag gets 412 and the current ETag.
	rec = send(t, h, http.MethodPut, "/api/config", body, map[string]string{"If-Match": etag})
	if rec.Code != http.StatusPreconditionFailed || rec.Header().Get("ETag") != newTag {
		t.Errorf("stale PUT = %d etag %q", rec.Code, rec.Header().Get("ETag"))
	}
	rec = send(t, h, http.MethodPut, "/api/config", `{"version":7}`, map[string]string{"If-Match": newTag})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid PUT = %d, want 400", rec.Code)
	}
}

func TestOverride(t *testing.T) {
	h, cfg, _ := apiServer(t)

	rec := send(t, h, http.MethodPut, "/api/overrides/docker:media/app", `{"pinned":true,"group":"media"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT override = %d %s", rec.Code, rec.Body)
	}
	if o := cfg.Config().Overrides["docker:media/app"]; !o.Pinned || o.Group != "media" {
		t.Errorf("override = %+v", o)
	}
	if rec := send(t, h, http.MethodPut, "/api/overrides/docker:media/app", `null`, nil); rec.Code != http.StatusOK {
		t.Errorf("clear override = %d", rec.Code)
	}
	if _, ok := cfg.Config().Overrides["docker:media/app"]; ok {
		t.Error("override not cleared")
	}
	if rec := send(t, h, http.MethodPut, "/api/overrides/docker:nope", `{}`, nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown service = %d, want 404", rec.Code)
	}
	if rec := send(t, h, http.MethodPut, "/api/overrides/docker:media/app", `[1]`, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("bad body = %d, want 400", rec.Code)
	}
}

func TestServicesList(t *testing.T) {
	h, _, _ := apiServer(t)
	var list []store.Service
	if err := json.Unmarshal(get(t, h, http.MethodGet, "/api/services").Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("services = %v %v", list, err)
	}
	// Check settings are server-only.
	if strings.Contains(get(t, h, http.MethodGet, "/api/services").Body.String(), "Check") {
		t.Error("check fields leaked to the browser")
	}
}

func TestEventsStream(t *testing.T) {
	h, _, reg := apiServer(t)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	lines := bufio.NewScanner(res.Body)
	next := func() (string, string) {
		var event, data string
		for lines.Scan() {
			line := lines.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			case line == "" && event != "":
				return event, data
			}
		}
		t.Fatal("stream ended")
		return "", ""
	}

	if ev, data := next(); ev != "snapshot" || !strings.Contains(data, `"docker:media/app"`) {
		t.Fatalf("first event = %s %s", ev, data)
	}
	reg.SetHealth("docker:media/app", store.Health{Status: "up", MS: 12})
	if ev, data := next(); ev != "health" || !strings.Contains(data, `"up"`) {
		t.Errorf("health event = %s %s", ev, data)
	}
	reg.Set([]store.Service{})
	if ev, _ := next(); ev != "service.remove" {
		t.Errorf("remove event = %s", ev)
	}
}

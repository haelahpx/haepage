package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/integrations"
	"github.com/haelahpx/haepage/internal/integrations/fakeapps"
	"github.com/haelahpx/haepage/internal/store"
)

func integrationServer(t *testing.T) http.Handler {
	t.Helper()
	apps := httptest.NewServer(fakeapps.Demo(time.Now()).Handler())
	t.Cleanup(apps.Close)
	reg := store.NewRegistry()
	reg.Set([]store.Service{{ID: "docker:media/jellyfin", Name: "Jellyfin", Integration: "jellyfin", URLs: store.URLs{LAN: apps.URL + "/jellyfin"}}})
	sec, err := config.OpenSecrets(filepath.Join(t.TempDir(), "secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := &integrations.Manager{Registry: reg, Secrets: sec, Kinds: integrations.Kinds(http.DefaultClient)}
	return New(Options{Static: built(), Registry: reg, Integrations: m})
}

func TestIntegrationAPI(t *testing.T) {
	h := integrationServer(t)
	if !strings.Contains(get(t, h, http.MethodGet, "/api/ping").Body.String(), `"integrations"`) {
		t.Error("integrations capability missing")
	}

	rec := send(t, h, http.MethodPost, "/api/integrations/connect", `{"service":"docker:media/jellyfin","key":"wrong"}`, nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "rejected") {
		t.Errorf("bad key = %d %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodPost, "/api/integrations/connect", `{"service":"nope","key":"k"}`, nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown service = %d", rec.Code)
	}
	rec = send(t, h, http.MethodPost, "/api/integrations/connect", `{"service":"docker:media/jellyfin","key":"`+fakeapps.Key+`"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("connect = %d %s", rec.Code, rec.Body)
	}

	var list []integrations.Info
	json.Unmarshal(get(t, h, http.MethodGet, "/api/integrations").Body.Bytes(), &list)
	if len(list) != 1 || !list[0].Connected || !strings.HasPrefix(list[0].Webhook, "/api/hooks/") {
		t.Fatalf("list = %+v", list)
	}
	if strings.Contains(get(t, h, http.MethodGet, "/api/integrations").Body.String(), fakeapps.Key) {
		t.Error("API key leaked in the integrations list")
	}

	img := get(t, h, http.MethodGet, "/api/integrations/image?service=docker:media/jellyfin&item=a1b2c3")
	if img.Code != http.StatusOK || !strings.HasPrefix(img.Header().Get("Content-Type"), "image/") ||
		!strings.Contains(img.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("image = %d %q csp %q", img.Code, img.Header().Get("Content-Type"), img.Header().Get("Content-Security-Policy"))
	}
	if rec := get(t, h, http.MethodGet, "/api/integrations/image?service=docker:media/jellyfin&item=../x"); rec.Code != http.StatusNotFound {
		t.Errorf("bad item = %d", rec.Code)
	}

	if rec := send(t, h, http.MethodPost, "/api/hooks/not-a-token", `{}`, nil); rec.Code != http.StatusNotFound {
		t.Errorf("bad webhook token = %d", rec.Code)
	}
	if rec := send(t, h, http.MethodPost, list[0].Webhook, `{}`, nil); rec.Code != http.StatusNoContent {
		t.Errorf("webhook = %d", rec.Code)
	}

	if rec := send(t, h, http.MethodPost, "/api/integrations/disconnect", `{"service":"docker:media/jellyfin"}`, nil); rec.Code != http.StatusOK {
		t.Errorf("disconnect = %d", rec.Code)
	}
	json.Unmarshal(get(t, h, http.MethodGet, "/api/integrations").Body.Bytes(), &list)
	if list[0].Connected {
		t.Error("still connected")
	}
}

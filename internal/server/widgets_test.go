package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/proxy"
)

// The M4 exit test: a widget secret never appears in any HTTP response
// body, whatever the API or the visitor does.
func TestSecretNeverInAnyResponse(t *testing.T) {
	const secret = "tok-ZXCVBNM-0042"
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "bad token "+r.Header.Get("Authorization"), http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"headers": r.Header, "url": r.URL.String(), "blocked": 7})
	}))
	defer echo.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "haepage.json")
	os.WriteFile(cfgPath, []byte(`{"version":1,"widgets":[
	  {"id":"echo","type":"api","options":{"url":"`+echo.URL+`/x?key={{secrets.tok}}","headers":{"Authorization":"Bearer {{secrets.tok}}"}}},
	  {"id":"fail","type":"api","options":{"url":"`+echo.URL+`/fail?key={{secrets.tok}}","headers":{"Authorization":"Bearer {{secrets.tok}}"}}},
	  {"id":"local","type":"api","options":{"url":"https://127.0.0.1:1/?key={{secrets.tok}}"}}
	]}`), 0o644)
	cfg, err := config.Open(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	secrets, _ := config.OpenSecrets(filepath.Join(dir, "secrets.json"))
	h := New(Options{
		Static:  built(),
		Config:  cfg,
		Secrets: secrets,
		Widgets: &proxy.Widgets{Config: cfg.Config, Secrets: secrets, Client: echo.Client()},
	})
	guarded := New(Options{Static: built(), Widgets: &proxy.Widgets{Config: cfg.Config, Secrets: secrets, Client: (&proxy.Guard{}).Client(2 * time.Second)}})

	// Set the secret through the API, as Settings does.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/secrets/tok", strings.NewReader(`{"value":"`+secret+`"}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT secret = %d %s", rec.Code, rec.Body)
	}

	responses := map[string]*httptest.ResponseRecorder{
		"ping":           get(t, h, http.MethodGet, "/api/ping"),
		"config":         get(t, h, http.MethodGet, "/api/config"),
		"haepage.json":   get(t, h, http.MethodGet, "/haepage.json"),
		"secrets":        get(t, h, http.MethodGet, "/api/secrets"),
		"echo widget":    get(t, h, http.MethodGet, "/api/widgets/echo/data"),
		"echo (cached)":  get(t, h, http.MethodGet, "/api/widgets/echo/data"),
		"failing widget": get(t, h, http.MethodGet, "/api/widgets/fail/data"),
		"blocked widget": get(t, guarded, http.MethodGet, "/api/widgets/local/data"),
		"no widget":      get(t, h, http.MethodGet, "/api/widgets/nope/data"),
	}
	for name, rec := range responses {
		body, _ := io.ReadAll(rec.Result().Body)
		if strings.Contains(string(body), secret) {
			t.Errorf("%s (%d) leaks the secret: %s", name, rec.Code, body)
		}
		for k, vs := range rec.Header() {
			if strings.Contains(strings.Join(vs, " "), secret) {
				t.Errorf("%s header %s leaks the secret", name, k)
			}
		}
	}

	// The echo widget still worked, with the secret scrubbed.
	if b := responses["echo widget"].Body.String(); responses["echo widget"].Code != 200 || !strings.Contains(b, `"blocked":7`) || !strings.Contains(b, "[secret]") {
		t.Errorf("echo widget = %d %s", responses["echo widget"].Code, b)
	}
	if responses["secrets"].Body.String() != "{\"names\":[\"tok\"]}\n" {
		t.Errorf("secrets list = %s", responses["secrets"].Body)
	}
	if c := responses["failing widget"].Code; c != http.StatusBadRequest {
		t.Errorf("failing widget = %d", c)
	}
	if c := responses["blocked widget"].Code; c != http.StatusBadRequest {
		t.Errorf("blocked widget = %d %s", c, responses["blocked widget"].Body)
	}
	if c := responses["no widget"].Code; c != http.StatusNotFound {
		t.Errorf("no widget = %d", c)
	}
}

func TestSecretsAPI(t *testing.T) {
	dir := t.TempDir()
	secrets, _ := config.OpenSecrets(filepath.Join(dir, "secrets.json"))
	secrets.Set("int:media", "integration-key")
	h := New(Options{Static: built(), Secrets: secrets})
	put := func(name, body string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/secrets/"+name, strings.NewReader(body)))
		return rec.Code
	}
	if c := put("BadName", `{"value":"x"}`); c != http.StatusBadRequest {
		t.Errorf("bad name = %d", c)
	}
	if c := put("int:media", `{"value":"x"}`); c != http.StatusBadRequest {
		t.Errorf("integration key name = %d", c)
	}
	if c := put("pihole", `{"value":"  abc  "}`); c != http.StatusNoContent || secrets.Get("pihole") != "abc" {
		t.Errorf("put = %d, value %q", c, secrets.Get("pihole"))
	}
	// Integration keys are never listed.
	if b := get(t, h, http.MethodGet, "/api/secrets").Body.String(); b != "{\"names\":[\"pihole\"]}\n" {
		t.Errorf("list = %s", b)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/secrets/pihole", nil))
	if rec.Code != http.StatusNoContent || secrets.Get("pihole") != "" {
		t.Errorf("delete = %d", rec.Code)
	}
}

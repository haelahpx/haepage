package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/proxy"
	"github.com/haelahpx/haepage/internal/themes"
)

func post(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

func TestThemeImport(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(":root{--accent:red}"))
	}))
	defer upstream.Close()
	h := New(Options{Static: built(), Themes: &themes.Remote{Dir: t.TempDir(), Client: upstream.Client()}})
	link := upstream.URL + "/red.css"

	if !strings.Contains(get(t, h, http.MethodGet, "/api/ping").Body.String(), `"themes.import"`) {
		t.Error("ping lacks themes.import")
	}
	if rec := get(t, h, http.MethodGet, "/api/themes/remote?url="+link); rec.Code != http.StatusNotFound {
		t.Fatalf("remote before import = %d", rec.Code)
	}
	rec := post(t, h, "/api/themes/import", `{"url":"`+link+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("import = %d %s", rec.Code, rec.Body)
	}
	var info themes.Info
	json.Unmarshal(rec.Body.Bytes(), &info)

	rec = get(t, h, http.MethodGet, "/api/themes/remote?url="+link)
	if rec.Code != http.StatusOK || rec.Body.String() != ":root{--accent:red}" {
		t.Fatalf("remote = %d %q", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q", ct)
	}
	if rec.Header().Get("X-Theme-Sha256") != info.SHA256 {
		t.Error("sha256 header doesn't match the import")
	}

	for _, bad := range []string{`{}`, `nope`, `{"url":"http://example.com/t.css"}`} {
		if rec := post(t, h, "/api/themes/import", bad); rec.Code != http.StatusBadRequest {
			t.Errorf("import %s = %d, want 400", bad, rec.Code)
		}
	}
}

func TestThemeImportIsGuarded(t *testing.T) {
	guard := &proxy.Guard{}
	h := New(Options{Static: built(), Themes: &themes.Remote{Dir: t.TempDir(), Client: guard.Client(2 * time.Second)}})
	for _, link := range []string{"https://127.0.0.1/t.css", "https://[::1]/t.css", "https://169.254.169.254/latest", "https://192.168.1.1/t.css"} {
		rec := post(t, h, "/api/themes/import", `{"url":"`+link+`"}`)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "not allowed") {
			t.Errorf("import %s = %d %s, want a blocked error", link, rec.Code, rec.Body)
		}
	}
}

func TestLimiter(t *testing.T) {
	l := newLimiter(2)
	now := time.Unix(0, 0)
	l.now = func() time.Time { return now }
	if !l.allow("a") || !l.allow("a") || l.allow("a") {
		t.Fatal("burst of 2 not enforced")
	}
	if !l.allow("b") {
		t.Fatal("limits must be per client")
	}
	now = now.Add(30 * time.Second) // 2 per minute → one token back
	if !l.allow("a") || l.allow("a") {
		t.Fatal("refill not at the configured rate")
	}
}

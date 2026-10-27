package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/config"
)

type secretMap map[string]string

func (s secretMap) Get(name string) string { return s[name] }

func apiWidget(id, url string, headers map[string]string, refresh int) config.Widget {
	opts, _ := json.Marshal(APIOptions{URL: url, Headers: headers})
	return config.Widget{ID: id, Type: "api", Refresh: refresh, Options: opts}
}

func TestDataFillsSecretsAndCaches(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("Authorization") != "Bearer tok-123456" || r.URL.Query().Get("key") != "q-987654" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"stats":{"blocked":42}}`))
	}))
	defer upstream.Close()

	widgets := []config.Widget{apiWidget("pi", upstream.URL+"/api?key={{secrets.qkey}}", map[string]string{"Authorization": "Bearer {{ secrets.token }}"}, 30)}
	now := time.Unix(0, 0)
	p := &Widgets{
		Config:  func() config.Config { return config.Config{Widgets: widgets} },
		Secrets: secretMap{"token": "tok-123456", "qkey": "q-987654"},
		Client:  upstream.Client(),
		now:     func() time.Time { return now },
	}
	body, err := p.Data(context.Background(), "pi")
	if err != nil || string(body) != `{"stats":{"blocked":42}}` {
		t.Fatalf("Data = %s, %v", body, err)
	}
	p.Data(context.Background(), "pi")
	if hits != 1 {
		t.Errorf("hits = %d, want 1 while cached", hits)
	}
	now = now.Add(31 * time.Second)
	p.Data(context.Background(), "pi")
	if hits != 2 {
		t.Errorf("hits = %d, want 2 after the refresh time", hits)
	}
	// Changing the widget's settings skips the cache.
	widgets[0] = apiWidget("pi", upstream.URL+"/api?key={{secrets.qkey}}&v=2", map[string]string{"Authorization": "Bearer {{secrets.token}}"}, 30)
	p.Data(context.Background(), "pi")
	if hits != 3 {
		t.Errorf("hits = %d, want 3 after a settings change", hits)
	}
}

func TestDataErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html></html>"))
		case "/fail":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer upstream.Close()
	widgets := []config.Widget{
		apiWidget("html", upstream.URL+"/html", nil, 0),
		apiWidget("fail", upstream.URL+"/fail", nil, 0),
		apiWidget("nosecret", upstream.URL+"/?k={{secrets.missing}}", nil, 0),
		apiWidget("intkey", upstream.URL+"/?k={{secrets.int:media}}", nil, 0),
		apiWidget("ftp", "ftp://example.com/x", nil, 0),
		{ID: "clock", Type: "clock"},
	}
	p := &Widgets{Config: func() config.Config { return config.Config{Widgets: widgets} }, Secrets: secretMap{"int:media": "integration-key"}, Client: upstream.Client()}
	for id, want := range map[string]error{"html": ErrWidget, "fail": ErrWidget, "nosecret": ErrWidget, "ftp": ErrWidget, "clock": ErrNoWidget, "gone": ErrNoWidget} {
		if _, err := p.Data(context.Background(), id); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", id, err, want)
		}
	}
	// "int:media" isn't a valid widget secret name, so it is left as text
	// and the integration key is never sent.
	body, err := p.Data(context.Background(), "intkey")
	if err != nil || strings.Contains(string(body), "integration-key") {
		t.Errorf("intkey = %s, %v", body, err)
	}
}

func TestSecretsAreRedacted(t *testing.T) {
	const secret = "s3cr3t-value-0001"
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An API that echoes what it was sent.
		json.NewEncoder(w).Encode(map[string]string{"auth": r.Header.Get("X-Key"), "url": r.URL.String()})
	}))
	defer echo.Close()
	widgets := []config.Widget{
		apiWidget("echo", echo.URL+"/?key={{secrets.k}}", map[string]string{"X-Key": "{{secrets.k}}"}, 0),
		apiWidget("blocked", "https://127.0.0.1/?key={{secrets.k}}", nil, 0),
	}
	cfg := func() config.Config { return config.Config{Widgets: widgets} }

	p := &Widgets{Config: cfg, Secrets: secretMap{"k": secret}, Client: echo.Client()}
	body, err := p.Data(context.Background(), "echo")
	if err != nil || strings.Contains(string(body), secret) || !strings.Contains(string(body), "[secret]") {
		t.Errorf("echoed answer = %s, %v", body, err)
	}

	guarded := &Widgets{Config: cfg, Secrets: secretMap{"k": secret}, Client: (&Guard{}).Client(2 * time.Second)}
	_, err = guarded.Data(context.Background(), "blocked")
	if !errors.Is(err, ErrBlocked) || strings.Contains(err.Error(), secret) {
		t.Errorf("blocked error = %v", err)
	}
}

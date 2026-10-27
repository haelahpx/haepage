package themes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodCSS = ":root { --accent: #e5484d; }\n"

func newRemote(t *testing.T, handler http.HandlerFunc) (*Remote, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	return &Remote{Dir: t.TempDir(), Client: srv.Client()}, srv
}

func TestImportCachesAndPins(t *testing.T) {
	body := goodCSS
	hits := 0
	r, srv := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(body))
	})
	link := srv.URL + "/red.css"

	if _, _, err := r.Get(link); !errors.Is(err, ErrNotImported) {
		t.Fatalf("Get before import: %v", err)
	}
	info, err := r.Import(context.Background(), link, false)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != len(goodCSS) || len(info.SHA256) != 64 || info.URL != link {
		t.Fatalf("info = %+v", info)
	}
	css, _, err := r.Get(link)
	if err != nil || string(css) != goodCSS {
		t.Fatalf("Get = %q, %v", css, err)
	}

	// The copy is pinned: the upstream file changing doesn't change it.
	body = ":root { --accent: #000; }"
	again, err := r.Import(context.Background(), link, false)
	if err != nil || again.SHA256 != info.SHA256 || hits != 1 {
		t.Fatalf("second import = %+v, %v (hits %d)", again, err, hits)
	}
	// Refresh fetches the new version.
	fresh, err := r.Import(context.Background(), link, true)
	if err != nil || fresh.SHA256 == info.SHA256 || hits != 2 {
		t.Fatalf("refresh = %+v, %v (hits %d)", fresh, err, hits)
	}

	// A tampered cache file is not served.
	cssPath, _ := r.paths(link)
	os.WriteFile(cssPath, []byte("body{display:none}"), 0o644)
	if _, _, err := r.Get(link); !errors.Is(err, ErrNotImported) {
		t.Fatalf("tampered cache: %v", err)
	}
}

func TestImportRejects(t *testing.T) {
	cases := map[string]struct {
		ctype, body string
		status      int
	}{
		"html page":     {"text/html", "<!doctype html><title>x</title>", 200},
		"html as plain": {"text/plain", "  <html></html>", 200},
		"json":          {"application/json", "{}", 200},
		"import":        {"text/css", "@import url(https://evil.example/x.css);", 200},
		"empty":         {"text/css", "", 200},
		"not found":     {"text/css", "", 404},
		"too big":       {"text/css", strings.Repeat("a", MaxSize+1), 200},
		"binary":        {"text/css", "a\x00b", 200},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r, srv := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", c.ctype)
				w.WriteHeader(c.status)
				w.Write([]byte(c.body))
			})
			if _, err := r.Import(context.Background(), srv.URL+"/t.css", false); err == nil {
				t.Fatal("import succeeded")
			}
			if entries, _ := os.ReadDir(r.Dir); len(entries) != 0 {
				t.Fatalf("rejected theme was cached: %v", entries)
			}
		})
	}
}

func TestCheckURL(t *testing.T) {
	for _, bad := range []string{"http://example.com/t.css", "file:///etc/passwd", "https://", "https://user:pw@example.com/t.css", "javascript:alert(1)"} {
		if _, err := CheckURL(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("CheckURL(%q) = %v", bad, err)
		}
	}
	if _, err := CheckURL("https://example.com/t.css"); err != nil {
		t.Error(err)
	}
	r := &Remote{Dir: filepath.Join(t.TempDir(), "x")}
	if _, err := r.Import(context.Background(), "http://example.com/t.css", false); !errors.Is(err, ErrInvalid) {
		t.Errorf("Import with http: %v", err)
	}
}

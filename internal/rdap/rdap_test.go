package rdap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeRegistry(t *testing.T, hits *int) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		switch r.URL.Path {
		case "/dns.json":
			w.Write([]byte(`{"services":[[["com","net"],["` + srv.URL + `/com/"]],[["dev"],["` + srv.URL + `/dev"]]]}`))
		case "/com/domain/example.com":
			w.Write([]byte(`{"events":[{"eventAction":"registration","eventDate":"1995-08-14T04:00:00Z"},
			  {"eventAction":"expiration","eventDate":"2027-08-13T04:00:00Z"}],
			  "entities":[{"roles":["registrar"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Example Registrar, Inc."]]]}]}`))
		case "/com/domain/noexpiry.com":
			w.Write([]byte(`{"events":[]}`))
		case "/dev/domain/home.dev":
			w.Write([]byte(`{"events":[{"eventAction":"expiration","eventDate":"2026-11-05T00:00:00+07:00"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLookup(t *testing.T) {
	hits := 0
	srv := fakeRegistry(t, &hits)
	c := &Client{HTTP: srv.Client(), Bootstrap: srv.URL + "/dns.json"}
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }

	d := c.Lookup(context.Background(), " Example.COM. ")
	if d.Error != "" || d.Expires == nil || d.Expires.Year() != 2027 || d.Registrar != "Example Registrar, Inc." || d.Name != "example.com" {
		t.Fatalf("example.com = %+v", d)
	}
	// A subdomain falls back to its registered domain.
	if d := c.Lookup(context.Background(), "nas.home.dev"); d.Expires == nil || d.Expires.Hour() != 17 {
		t.Errorf("nas.home.dev = %+v", d)
	}
	for name, want := range map[string]string{
		"noexpiry.com":   "doesn't publish",
		"missing.com":    "doesn't know",
		"example.museum": "doesn't offer RDAP",
		"not a domain":   "not a domain",
		"localhost":      "not a domain",
	} {
		if d := c.Lookup(context.Background(), name); !strings.Contains(d.Error, want) {
			t.Errorf("%s = %+v, want error with %q", name, d, want)
		}
	}

	// Answers and the directory are cached.
	before := hits
	c.Lookup(context.Background(), "example.com")
	if hits != before {
		t.Errorf("cached lookup made %d requests", hits-before)
	}
	now = now.Add(13 * time.Hour)
	c.Lookup(context.Background(), "example.com")
	if hits != before+1 {
		t.Errorf("expired answer: %d requests, want 1 (the directory is still fresh)", hits-before)
	}
}

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

func feedWidget(id, typ string, opts FeedOptions) config.Widget {
	raw, _ := json.Marshal(opts)
	return config.Widget{ID: id, Type: typ, Options: raw}
}

func TestFeedWidgets(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/blog.xml":
			w.Write([]byte(`<rss version="2.0"><channel><title>Blog</title>
			  <item><title>One</title><link>https://b.example/1</link><pubDate>Mon, 26 Oct 2026 08:00:00 +0000</pubDate></item>
			  <item><title>Two</title><link>https://b.example/2</link><pubDate>Tue, 27 Oct 2026 08:00:00 +0000</pubDate></item>
			</channel></rss>`))
		case r.URL.Path == "/cal.ics" && r.URL.Query().Get("token") == "cal-secret-99":
			w.Write([]byte("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:1\r\nSUMMARY:Backup day\r\nDTSTART:20261028T100000Z\r\nEND:VEVENT\r\n" +
				"BEGIN:VEVENT\r\nUID:2\r\nSUMMARY:Next year\r\nDTSTART:20271028T100000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"))
		case strings.HasSuffix(r.URL.Path, "/releases.atom") && strings.HasPrefix(r.URL.Path, "/org/app"):
			w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"><title>app</title>
			  <entry><title>v2.0</title><link href="https://code.example/org/app/v2.0"/><updated>2026-10-26T00:00:00Z</updated></entry></feed>`))
		case strings.HasSuffix(r.URL.Path, "/releases.atom") && strings.HasPrefix(r.URL.Path, "/org/lib"):
			w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"><title>lib</title>
			  <entry><title>v0.9</title><link href="https://code.example/org/lib/v0.9"/><updated>2026-10-27T00:00:00Z</updated></entry></feed>`))
		case r.URL.Path == "/page":
			w.Write([]byte("<html><body>not a feed</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	widgets := []config.Widget{
		feedWidget("blog", "rss", FeedOptions{URL: upstream.URL + "/blog.xml", Limit: 1}),
		feedWidget("cal", "calendar", FeedOptions{URL: upstream.URL + "/cal.ics?token={{secrets.cal}}", Days: 7}),
		feedWidget("rel", "releases", FeedOptions{Repos: []string{"org/app", "https://github.com/org/lib/", "org/missing", "not a repo"}}),
		feedWidget("notfeed", "rss", FeedOptions{URL: upstream.URL + "/page"}),
		feedWidget("notcal", "calendar", FeedOptions{URL: upstream.URL + "/page"}),
	}
	p := &Widgets{
		Config:      func() config.Config { return config.Config{Widgets: widgets} },
		Secrets:     secretMap{"cal": "cal-secret-99"},
		Client:      upstream.Client(),
		now:         func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) },
		releaseBase: upstream.URL,
	}

	var blog struct {
		Title string
		Items []struct{ Title, Link string }
	}
	body, err := p.Data(context.Background(), "blog")
	if json.Unmarshal(body, &blog); err != nil || blog.Title != "Blog" || len(blog.Items) != 1 || blog.Items[0].Title != "Two" {
		t.Errorf("rss = %s, %v", body, err)
	}

	var cal struct{ Events []struct{ Title string } }
	body, err = p.Data(context.Background(), "cal")
	if json.Unmarshal(body, &cal); err != nil || len(cal.Events) != 1 || cal.Events[0].Title != "Backup day" {
		t.Errorf("calendar = %s, %v", body, err)
	}
	if strings.Contains(string(body), "cal-secret-99") {
		t.Error("calendar answer leaks the secret")
	}

	var rel struct {
		Items  []struct{ Repo, Title string }
		Errors []struct{ Repo string }
	}
	body, err = p.Data(context.Background(), "rel")
	if json.Unmarshal(body, &rel); err != nil || len(rel.Items) != 2 || rel.Items[0].Repo != "org/lib" || rel.Items[1].Title != "v2.0" || len(rel.Errors) != 2 {
		t.Errorf("releases = %s, %v", body, err)
	}

	for _, id := range []string{"notfeed", "notcal"} {
		if _, err := p.Data(context.Background(), id); !errors.Is(err, ErrWidget) {
			t.Errorf("%s: err = %v", id, err)
		}
	}
}

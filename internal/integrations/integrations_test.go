package integrations

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/integrations/fakeapps"
	"github.com/haelahpx/haepage/internal/store"
)

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func fake(t *testing.T) (*fakeapps.Apps, string) {
	t.Helper()
	apps := fakeapps.Demo(now)
	srv := httptest.NewServer(apps.Handler())
	t.Cleanup(srv.Close)
	return apps, srv.URL
}

var ctx = context.Background()

func TestJellyfin(t *testing.T) {
	apps, base := fake(t)
	j := Kinds(http.DefaultClient)["jellyfin"].(*Jellyfin)
	c := Conn{BaseURL: base + "/jellyfin", Key: fakeapps.Key}

	if err := j.Test(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := j.Test(ctx, Conn{BaseURL: c.BaseURL, Key: "wrong"}); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("bad key: %v", err)
	}
	got, err := j.Poll(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	s := got.(JellyfinData).Sessions
	if len(s) != 2 {
		t.Fatalf("sessions = %+v (the idle session must be skipped)", s)
	}
	if s[0].Title != "Northern Lights" || s[0].Subtitle != "S02E04 · The Long Night" || s[0].PositionMS != 23*60_000 || s[0].Transcoding {
		t.Errorf("episode = %+v", s[0])
	}
	if s[1].Title != "Ocean Drift" || s[1].Subtitle != "2024" || !s[1].Transcoding {
		t.Errorf("movie = %+v", s[1])
	}

	apps.Lock()
	apps.Sessions = append(apps.Sessions[:1], fakeapps.Session{ID: "s3", User: "Jo", ItemID: "zz", Name: "New Film", Type: "Movie"})
	apps.Unlock()
	next, _ := j.Poll(ctx, c)
	acts := j.Diff(got, next)
	if len(acts) != 1 || acts[0].Type != "playback.start" || acts[0].Title != "Jo started watching New Film" {
		t.Errorf("diff = %+v", acts)
	}

	img, ctype, err := j.Image(ctx, c, "a1b2c3")
	if err != nil || ctype != "image/svg+xml" || !strings.Contains(string(img), "<svg") {
		t.Errorf("image = %q %q %v", img, ctype, err)
	}
	if _, _, err := j.Image(ctx, c, "../System/Info"); err == nil {
		t.Error("path traversal in item id accepted")
	}
}

func TestSeerr(t *testing.T) {
	apps, base := fake(t)
	s := Kinds(http.DefaultClient)["seerr"].(*Seerr)
	c := Conn{BaseURL: base + "/seerr", Key: fakeapps.Key}

	if err := s.Test(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.Poll(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	d := got.(SeerrData)
	if d.Pending != 2 || len(d.Requests) != 2 || d.Requests[0].Title != "Harbor City" || d.Requests[0].RequestedBy != "Jo" ||
		d.Requests[0].Poster != tmdbPoster+"/fake2002.jpg" || d.Requests[1].Title != "Starfall" {
		t.Errorf("data = %+v", d)
	}
	// Titles are looked up once per TMDB id.
	s.Poll(ctx, c)
	apps.Lock()
	lookups := apps.Calls["/seerr/api/v1/movie/1001"]
	apps.Unlock()
	if lookups != 1 {
		t.Errorf("title looked up %d times, want 1", lookups)
	}

	apps.Lock()
	apps.Requests = append(apps.Requests, fakeapps.Request{ID: 43, Type: "movie", TmdbID: 1003, Title: "Paper Moon", RequestedBy: "Alex", CreatedAt: now})
	apps.Unlock()
	next, _ := s.Poll(ctx, c)
	if acts := s.Diff(got, next); len(acts) != 1 || acts[0].Title != "Alex requested Paper Moon" || acts[0].Type != "request.new" {
		t.Errorf("diff = %+v", acts)
	}
}

func TestSeerrWebhook(t *testing.T) {
	s := &Seerr{}
	cases := map[string]string{
		`{"notification_type":"MEDIA_PENDING","subject":"Dune (2021)","request":{"requestedBy_username":"sam"}}`: "request.new:sam requested Dune (2021)",
		`{"notification_type":"MEDIA_AVAILABLE","subject":"Dune (2021)"}`:                                        "request.available:Dune (2021) is ready to watch",
		`{"notification_type":"MEDIA_AUTO_APPROVED","subject":"X"}`:                                              "request.approved:X was approved",
		`{"notification_type":"TEST_NOTIFICATION"}`:                                                              "test:Webhook connected",
	}
	for body, want := range cases {
		acts := s.Webhook([]byte(body))
		if len(acts) != 1 || acts[0].Type+":"+acts[0].Title != want {
			t.Errorf("%s → %+v", body, acts)
		}
	}
	for _, body := range []string{`{"notification_type":"MEDIA_DECLINED"}`, `not json`} {
		if acts := s.Webhook([]byte(body)); acts != nil {
			t.Errorf("%s → %+v, want nothing", body, acts)
		}
	}
}

func TestArr(t *testing.T) {
	apps, base := fake(t)
	kinds := Kinds(http.DefaultClient)
	radarr := kinds["radarr"].(*Arr)
	sonarr := kinds["sonarr"].(*Arr)
	radarr.now, sonarr.now = func() time.Time { return now }, func() time.Time { return now }

	if err := radarr.Test(ctx, Conn{BaseURL: base + "/sonarr", Key: fakeapps.Key}); err == nil || !strings.Contains(err.Error(), "this is Sonarr, not Radarr") {
		t.Errorf("wrong app accepted: %v", err)
	}
	r, err := radarr.Poll(ctx, Conn{BaseURL: base + "/radarr", Key: fakeapps.Key})
	if err != nil {
		t.Fatal(err)
	}
	rd := r.(ArrData)
	if len(rd.Queue) != 1 || rd.Queue[0].Title != "Starfall" || rd.Queue[0].Progress != 64.6 || len(rd.Upcoming) != 1 || len(rd.Health) != 0 {
		t.Errorf("radarr = %+v", rd)
	}

	sc := Conn{BaseURL: base + "/sonarr", Key: fakeapps.Key}
	s, err := sonarr.Poll(ctx, sc)
	if err != nil {
		t.Fatal(err)
	}
	sd := s.(ArrData)
	if sd.Queue[0].Subtitle != "S01E01 · Arrival" || sd.Upcoming[0].Title != "Northern Lights" || sd.Upcoming[0].Subtitle != "S02E05 · Thaw" {
		t.Errorf("sonarr = %+v", sd)
	}
	if len(sd.Health) != 1 || sd.Health[0].Type != "warning" {
		t.Errorf("health = %+v (notices are dropped)", sd.Health)
	}

	apps.Lock()
	apps.Sonarr.Queue = append(apps.Sonarr.Queue, fakeapps.QueueItem{ID: 10, Title: "Harbor City", Season: 1, Episode: 2, SizeMB: 1, LeftMB: 1})
	apps.Sonarr.Health = append(apps.Sonarr.Health, "Disk is almost full")
	apps.Unlock()
	next, _ := sonarr.Poll(ctx, sc)
	acts := sonarr.Diff(s, next)
	if len(acts) != 2 || acts[0].Type != "download.grabbed" || acts[1].Type != "health.warning" || acts[1].Detail != "Disk is almost full" {
		t.Errorf("diff = %+v", acts)
	}
}

func TestArrWebhook(t *testing.T) {
	a := &Arr{App: "sonarr"}
	acts := a.Webhook([]byte(`{"eventType":"Download","series":{"title":"Harbor City"},"episodes":[{"seasonNumber":1,"episodeNumber":2,"title":"Tides"}]}`))
	if len(acts) != 1 || acts[0].Type != "download.imported" || acts[0].Title != "Harbor City is ready to watch" || acts[0].Detail != "S01E02 · Tides" {
		t.Errorf("download = %+v", acts)
	}
	if acts := a.Webhook([]byte(`{"eventType":"Grab","movie":{"title":"Dune"}}`)); acts[0].Title != "Dune is downloading" {
		t.Errorf("grab = %+v", acts)
	}
	if acts := a.Webhook([]byte(`{"eventType":"Rename"}`)); acts != nil {
		t.Errorf("unhandled event produced %+v", acts)
	}
}

// manager wires a Manager to a registry with one service per fake app.
func manager(t *testing.T, base string) (*Manager, *store.Registry, *config.Secrets) {
	t.Helper()
	reg := store.NewRegistry()
	reg.Set([]store.Service{
		{ID: "docker:media/jellyfin", Name: "Jellyfin", Integration: "jellyfin", URLs: store.URLs{LAN: base + "/jellyfin"}},
		{ID: "docker:media/seerr", Name: "Requests", Integration: "seerr", URLs: store.URLs{LAN: base + "/seerr"}},
		{ID: "plain", Name: "Plain", URLs: store.URLs{LAN: base}},
	})
	sec, err := config.OpenSecrets(filepath.Join(t.TempDir(), "secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	clock := now
	m := &Manager{Registry: reg, Secrets: sec, Kinds: Kinds(http.DefaultClient), now: func() time.Time { return clock }}
	return m, reg, sec
}

func TestManagerConnectPollAndWebhook(t *testing.T) {
	apps, base := fake(t)
	m, reg, sec := manager(t, base)

	if err := m.Connect(ctx, "docker:media/jellyfin", "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("bad key: %v", err)
	}
	if sec.Get(keyName("docker:media/jellyfin")) != "" {
		t.Error("a rejected key was saved")
	}
	if err := m.Connect(ctx, "plain", "x"); !errors.Is(err, ErrNoSupport) {
		t.Errorf("plain service: %v", err)
	}
	if err := m.Connect(ctx, "docker:media/seerr", fakeapps.Key); err != nil {
		t.Fatal(err)
	}

	// Nobody is watching: no polls.
	m.Tick(ctx)
	apps.Lock()
	calls := apps.Calls["/seerr/api/v1/request"]
	apps.Unlock()
	if calls != 0 {
		t.Errorf("polled %d times with nobody watching", calls)
	}

	ch, stop := reg.Subscribe()
	defer stop()
	m.Tick(ctx)
	apps.Lock()
	apps.Requests = append(apps.Requests, fakeapps.Request{ID: 50, Type: "movie", TmdbID: 1050, Title: "Night Train", RequestedBy: "Kim"})
	apps.Unlock()
	m.mu.Lock()
	m.conns["docker:media/seerr"].next = time.Time{}
	m.mu.Unlock()
	m.Tick(ctx)

	var gotState, gotActivity bool
	for len(ch) > 0 {
		e := <-ch
		switch e.Type {
		case "integration":
			st := e.Data.(State)
			gotState = gotState || (st.Status == "ok" && st.Data.(SeerrData).Pending >= 2)
		case "activity":
			a := e.Data.(Activity)
			gotActivity = a.Title == "Kim requested Night Train" && a.Service == "docker:media/seerr" && a.Kind == "seerr" && a.ID != ""
		}
	}
	if !gotState || !gotActivity {
		t.Errorf("state published: %v, activity published: %v", gotState, gotActivity)
	}

	// Webhooks: a wrong token is refused; the right one adds activity and,
	// from then on, polls stop reporting the same events twice.
	if m.Webhook("nope", []byte(`{}`)) {
		t.Error("unknown token accepted")
	}
	list := m.List()
	var hook string
	for _, info := range list {
		if info.Service == "docker:media/seerr" {
			hook = strings.TrimPrefix(info.Webhook, "/api/hooks/")
		}
	}
	if len(list) != 2 || hook == "" {
		t.Fatalf("list = %+v", list)
	}
	if !m.Webhook(hook, []byte(`{"notification_type":"MEDIA_AVAILABLE","subject":"Night Train"}`)) {
		t.Fatal("webhook refused")
	}
	if !m.conns["docker:media/seerr"].hooked {
		t.Error("connection not marked as webhook-driven")
	}

	if err := m.Disconnect("docker:media/seerr"); err != nil {
		t.Fatal(err)
	}
	if sec.Get(keyName("docker:media/seerr")) != "" || sec.Get(hookName("docker:media/seerr")) != "" {
		t.Error("secrets kept after disconnect")
	}
}

func TestManagerReportsErrorsAndBacksOff(t *testing.T) {
	_, base := fake(t)
	m, reg, sec := manager(t, base)
	sec.Set(keyName("docker:media/jellyfin"), "revoked-key") // saved earlier, now wrong
	_, stop := reg.Subscribe()
	defer stop()

	m.Tick(ctx) // picks up the saved key and polls
	st := reg.State()["integrations"].([]any)
	if len(st) != 1 || st[0].(State).Status != "error" || !strings.Contains(st[0].(State).Error, "rejected") {
		t.Fatalf("state = %+v", st)
	}
	if next := m.conns["docker:media/jellyfin"].next; next.Sub(now) != 10*time.Second {
		t.Errorf("first retry in %v, want 10s", next.Sub(now))
	}
}

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/store"
)

func apps(t *testing.T) *KnownApps {
	t.Helper()
	k, err := LoadKnownApps()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestNormalizeImage(t *testing.T) {
	cases := map[string]string{
		"lscr.io/linuxserver/radarr:latest":   "linuxserver/radarr",
		"ghcr.io/hotio/sonarr:release-4.0":    "hotio/sonarr",
		"jellyfin/jellyfin:10.10@sha256:abcd": "jellyfin/jellyfin",
		"library/nextcloud:28":                "nextcloud",
		"nextcloud":                           "nextcloud",
		"localhost:5000/team/app:dev":         "team/app",
		"registry.example.com:5000/x/y":       "x/y",
		" Traefik:v3 ":                        "traefik",
	}
	for in, want := range cases {
		if got := NormalizeImage(in); got != want {
			t.Errorf("NormalizeImage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKnownAppsMatch(t *testing.T) {
	k := apps(t)
	cases := map[string]string{
		"lscr.io/linuxserver/jellyfin:latest": "jellyfin", // exact
		"jellyfin/jellyfin":                   "jellyfin",
		"ghcr.io/someone/jellyfin:nightly":    "jellyfin", // fork, matched by last segment
		"ghcr.io/hotio/radarr":                "radarr",
		"unknown/thing":                       "",
	}
	for img, want := range cases {
		got := ""
		if a := k.Match(img); a != nil {
			got = a.ID
		}
		if got != want {
			t.Errorf("Match(%q) = %q, want %q", img, got, want)
		}
	}
}

func TestKnownAppsFileIsValid(t *testing.T) {
	var file struct {
		Apps []App `json:"apps"`
	}
	if err := json.Unmarshal(knownAppsJSON, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Apps) < 40 {
		t.Errorf("only %d known apps, want 40 or more", len(file.Apps))
	}
	ids := map[string]bool{}
	for _, a := range file.Apps {
		if a.ID == "" || a.Name == "" || len(a.Images) == 0 || a.Group == "" {
			t.Errorf("incomplete entry: %+v", a)
		}
		if ids[a.ID] {
			t.Errorf("duplicate id %q", a.ID)
		}
		ids[a.ID] = true
	}
}

type port struct {
	IP      string
	Private int
	Public  int
}

func container(name, image, state string, labels map[string]string, ports ...port) Container {
	c := Container{ID: "abcdef1234567890", Names: []string{"/" + name}, Image: image, State: state, Labels: labels}
	for _, p := range ports {
		c.Ports = append(c.Ports, struct {
			IP          string `json:"IP"`
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		}{p.IP, p.Private, p.Public, "tcp"})
	}
	return c
}

func TestFromContainer(t *testing.T) {
	k := apps(t)
	compose := map[string]string{"com.docker.compose.project": "media", "com.docker.compose.service": "jellyfin"}

	d, ok := FromContainer(container("media-jellyfin-1", "lscr.io/linuxserver/jellyfin", "running", compose,
		port{"0.0.0.0", 8920, 8920}, port{"0.0.0.0", 8096, 18096}), k)
	if !ok {
		t.Fatal("jellyfin not detected")
	}
	if d.Key != "docker:media/jellyfin" || d.Name != "Jellyfin" || d.Group != "media" || d.Integration != "jellyfin" {
		t.Errorf("detected = %+v", d)
	}
	// The known app's port (8096 inside) wins over the lower 8920.
	if d.Port != 18096 || d.URLs.LAN != "http://{host}:18096" || d.CheckURL != "http://{host}:18096/health" {
		t.Errorf("port = %d urls = %+v check = %s", d.Port, d.URLs, d.CheckURL)
	}
	if d.Running == nil || !*d.Running {
		t.Error("running not set")
	}
}

func TestFromContainerLabelsWin(t *testing.T) {
	labels := map[string]string{
		"haepage.id": "movies", "haepage.name": "Movies", "haepage.group": "Watch",
		"dash.icon": "film", "haepage.url.public": "https://movies.example.com", "haepage.check": "none",
	}
	d, ok := FromContainer(container("x", "jellyfin/jellyfin", "exited", labels), apps(t))
	if !ok {
		t.Fatal("labelled container without ports should still appear")
	}
	if d.Key != "label:movies" || d.Name != "Movies" || d.Group != "watch" || d.Icon != "film" {
		t.Errorf("detected = %+v", d)
	}
	if d.URLs.Public != "https://movies.example.com" || d.URLs.LAN != "" || d.CheckType != "none" {
		t.Errorf("urls = %+v check = %s", d.URLs, d.CheckType)
	}
}

func TestFromContainerSkips(t *testing.T) {
	k := apps(t)
	cases := map[string]Container{
		"opted out":     container("a", "nginx", "running", map[string]string{"haepage.enable": "false"}, port{"0.0.0.0", 80, 8080}),
		"no ports":      container("db", "postgres", "running", nil),
		"loopback only": container("b", "nginx", "running", nil, port{"127.0.0.1", 80, 8080}),
		"haepage":       container("haepage", "ghcr.io/haelahpx/haepage:latest", "running", nil, port{"0.0.0.0", 8080, 8080}),
	}
	for name, c := range cases {
		if _, ok := FromContainer(c, k); ok {
			t.Errorf("%s: should be skipped", name)
		}
	}
}

func TestFromContainerUnknownApp(t *testing.T) {
	d, ok := FromContainer(container("spotify-lyrics-api", "someone/lyrics:latest", "running", nil,
		port{"0.0.0.0", 8080, 8080}, port{"::", 8080, 8080}), apps(t))
	if !ok || d.Key != "docker:spotify-lyrics-api" || d.Name != "Spotify Lyrics Api" || d.Group != "" || d.Port != 8080 {
		t.Errorf("detected = %+v ok = %v", d, ok)
	}
}

func TestMerge(t *testing.T) {
	cfg := config.Config{
		Services: []config.Service{
			{ID: "router", Name: "Router", Group: "net", URLs: config.URLs{LAN: "http://192.168.1.1"}},
			{ID: "media", Name: "Media", Group: "media", URLs: config.URLs{LAN: "http://10.0.0.5:18096"}},
		},
		Overrides: map[string]config.Override{
			"docker:a/pinned": {Pinned: true, Name: "Renamed", Group: "tools"},
			"docker:a/hidden": {Hidden: true},
			"docker:a/seen":   {Icon: "x"},
		},
	}
	running := true
	detected := []Detected{
		{Key: "docker:media/jellyfin", Name: "Jellyfin", Group: "media", Port: 18096, URLs: store.URLs{LAN: "http://{host}:18096"}},
		{Key: "docker:a/pinned", Name: "Orig", Group: "dev", Port: 1, Running: &running},
		{Key: "docker:a/hidden", Name: "Hidden", Port: 2},
		{Key: "docker:a/seen", Name: "Seen", Group: "dev", Port: 3},
		{Key: "docker:a/new", Name: "Brand New", Group: "dev", Port: 4},
	}
	isLocal := func(h string) bool { return h == "10.0.0.5" }

	got := Merge(cfg, detected, isLocal)
	byID := map[string]int{}
	for i, s := range got {
		byID[s.ID] = i
	}
	if got[0].ID != "router" || got[1].ID != "media" || got[0].State != "pinned" {
		t.Errorf("manual services not first: %v", got[:2])
	}
	if _, dup := byID["docker:media/jellyfin"]; dup {
		t.Error("detected service on a manual service's local port was not merged away")
	}
	pinned := got[byID["docker:a/pinned"]]
	if pinned.State != "pinned" || pinned.Name != "Renamed" || pinned.Group != "tools" || pinned.IsNew {
		t.Errorf("pinned = %+v", pinned)
	}
	if got[byID["docker:a/hidden"]].State != "hidden" {
		t.Error("hidden override ignored")
	}
	seen := got[byID["docker:a/seen"]]
	if seen.Group != InboxGroup || seen.IsNew || seen.Icon != "x" {
		t.Errorf("seen = %+v (an override without pin keeps it in the inbox, not new)", seen)
	}
	fresh := got[byID["docker:a/new"]]
	if fresh.Group != InboxGroup || !fresh.IsNew || fresh.Suggested != "dev" || fresh.State != "detected" {
		t.Errorf("new = %+v", fresh)
	}

	cfg.Settings.Inbox = "auto"
	got = Merge(cfg, detected, isLocal)
	for _, s := range got {
		if s.ID == "docker:a/new" && s.Group != "dev" {
			t.Errorf("auto placement: group = %q, want dev", s.Group)
		}
	}
}

func TestMergeChecks(t *testing.T) {
	cfg := config.Config{Services: []config.Service{
		{ID: "a", Name: "A", URLs: config.URLs{LAN: "http://{host}:9000"}, Check: &config.Check{Path: "/health", Expect: 204, Interval: 30}},
		{ID: "b", Name: "B", URLs: config.URLs{Public: "https://b.example.com"}, Check: &config.Check{Type: "none"}},
	}}
	got := Merge(cfg, nil, func(string) bool { return false })
	if got[0].CheckURL != "http://{host}:9000/health" || got[0].CheckExpect != 204 || got[0].CheckInterval != 30 {
		t.Errorf("a = %+v", got[0])
	}
	if got[1].CheckType != "none" || got[1].CheckURL != "https://b.example.com" {
		t.Errorf("b = %+v", got[1])
	}
}

const tcp = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1 1
   1: 0100007F:0CEA 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 2 1
   2: 0101A8C0:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 3 1
   3: 0101A8C0:1F90 0201A8C0:C350 01 00000000:00000000 00:00000000 00000000  1000        0 4 1
`
const tcp6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:2B66 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 5 1
   1: 00000000000000000000000001000000:1F91 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 6 1
   2: 0000000000000000FFFF00000100007F:1F92 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 7 1
`

func fakeProc(t *testing.T) string {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "proc/net"), 0o755)
	os.WriteFile(filepath.Join(root, "proc/net/tcp"), []byte(tcp), 0o644)
	os.WriteFile(filepath.Join(root, "proc/net/tcp6"), []byte(tcp6), 0o644)
	return root
}

func TestListeningPorts(t *testing.T) {
	// 8080 on all addresses, 3306 loopback, 22 on a LAN IP, a non-listening
	// socket; IPv6: 11110 on all, 8081 on ::1, 8082 on ::ffff:127.0.0.1.
	got := ListeningPorts(fakeProc(t))
	if fmt.Sprint(got) != "[22 8080 11110]" {
		t.Errorf("ports = %v", got)
	}
}

func TestPortScanner(t *testing.T) {
	probes := 0
	s := &PortScanner{
		Root: fakeProc(t),
		Probe: func(_ context.Context, port int) (string, bool) {
			probes++
			if port == 8080 {
				return "My App", true
			}
			return "", false // 11110 doesn't speak HTTP
		},
	}
	got := s.Scan(context.Background(), nil)
	if len(got) != 1 || got[0].Key != "port:8080" || got[0].Name != "My App" || got[0].URLs.LAN != "http://{host}:8080" {
		t.Fatalf("scan = %+v", got)
	}
	// 22 is skipped without probing; results are cached.
	s.Scan(context.Background(), nil)
	if probes != 2 {
		t.Errorf("probes = %d, want 2 (cached on the second scan)", probes)
	}
	if got := s.Scan(context.Background(), map[int]bool{8080: true}); len(got) != 0 {
		t.Errorf("skip list ignored: %+v", got)
	}
}

func TestPortScannerNamesUniqueAppPorts(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "proc/net"), 0o755)
	// 11434 (only one known app uses it) and 8080 (shared by several).
	os.WriteFile(filepath.Join(root, "proc/net/tcp"), []byte(`  sl  local_address rem_address   st
   0: 00000000:2CAA 00000000:0000 0A
   1: 00000000:1F90 00000000:0000 0A
`), 0o644)
	s := &PortScanner{Root: root, Apps: apps(t), Probe: func(context.Context, int) (string, bool) { return "", true }}
	got := s.Scan(context.Background(), nil)
	if len(got) != 2 || got[0].Name != "Port 8080" || got[1].Name != "Ollama" || got[1].Group != "tools" {
		t.Errorf("scan = %+v", got)
	}
}

func TestRunnerDropsPortsDockerPublishes(t *testing.T) {
	r := &Runner{}
	var got []Detected
	r.OnChange = func(d []Detected) { got = d }
	r.docker = []Detected{{Key: "docker:web", Port: 8080}}
	r.dockerPub = map[int]bool{8080: true}
	r.ports = []Detected{{Key: "port:8080", Port: 8080}, {Key: "port:9000", Port: 9000}}
	r.publish()
	if len(got) != 2 || got[1].Key != "port:9000" {
		t.Errorf("published = %+v", got)
	}
}

// fakeDocker serves the three Docker endpoints on a unix socket.
func fakeDocker(t *testing.T, containers string, events []string) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "docker.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /containers/json", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("all") != "1" {
			t.Error("containers listed without all=1")
		}
		w.Write([]byte(containers))
	})
	mux.HandleFunc("GET /containers/{id}/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"HostConfig":{"PortBindings":{"3000/tcp":[{"HostIp":"","HostPort":"18556"}],"53/udp":[{"HostPort":"53"}]}}}`))
	})
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("filters"), `"container"`) {
			t.Error("events not filtered to containers")
		}
		for _, e := range events {
			fmt.Fprintf(w, `{"Type":"container","Action":%q}`+"\n", e)
			w.(http.Flusher).Flush()
		}
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return "unix://" + sock
}

func TestDockerClient(t *testing.T) {
	host := fakeDocker(t, `[{"Id":"1","Names":["/web"],"Image":"nginx","State":"running","Labels":{},"Ports":[{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8088,"Type":"tcp"}]}]`,
		[]string{"start", "die"})
	c, err := NewDockerClient(host)
	if err != nil {
		t.Fatal(err)
	}
	list, err := c.Containers(context.Background())
	if err != nil || len(list) != 1 || list[0].Name() != "web" || list[0].Ports[0].PublicPort != 8088 {
		t.Fatalf("containers = %+v err = %v", list, err)
	}
	var actions []string
	err = c.Events(context.Background(), func(a string) { actions = append(actions, a) })
	if fmt.Sprint(actions) != "[start die]" || err == nil {
		t.Errorf("actions = %v, err = %v (the stream ending is an error)", actions, err)
	}
	if _, err := NewDockerClient("ftp://x"); err == nil {
		t.Error("bad scheme accepted")
	}
}

func TestRunnerPublishesDockerServices(t *testing.T) {
	host := fakeDocker(t, `[{"Id":"1","Names":["/web"],"Image":"nginx","State":"running","Labels":{},"Ports":[{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8088,"Type":"tcp"}]}]`, nil)
	c, _ := NewDockerClient(host)
	got := make(chan []Detected, 4)
	r := &Runner{Docker: c, Apps: apps(t), OnChange: func(d []Detected) { got <- d }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Run(ctx)
	select {
	case d := <-got:
		if len(d) != 1 || d[0].Key != "docker:web" {
			t.Errorf("detected = %+v", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runner published nothing")
	}
}

func TestRunnerKeepsStoppedContainers(t *testing.T) {
	// Docker lists a stopped container without ports; its configured
	// bindings keep it on the page, marked as not running.
	host := fakeDocker(t, `[{"Id":"2","Names":["/demo"],"Image":"node","State":"exited","Labels":{},"Ports":[]}]`, nil)
	c, _ := NewDockerClient(host)
	got := make(chan []Detected, 4)
	r := &Runner{Docker: c, Apps: apps(t), OnChange: func(d []Detected) { got <- d }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Run(ctx)
	select {
	case d := <-got:
		if len(d) != 1 || d[0].Port != 18556 || d[0].Running == nil || *d[0].Running {
			t.Errorf("detected = %+v", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runner published nothing")
	}
}

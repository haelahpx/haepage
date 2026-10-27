package tailscale

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const statusJSON = `{
  "BackendState": "Running",
  "CurrentTailnet": {"Name": "example.github"},
  "Self": {"HostName": "fedora", "DNSName": "fedora.tail1234.ts.net.", "OS": "linux",
           "TailscaleIPs": ["100.64.0.1", "fd7a::1"], "Online": false, "PublicKey": "nodekey:secret"},
  "Peer": {
    "a": {"HostName": "Pixel 8", "DNSName": "pixel-8.tail1234.ts.net.", "OS": "android",
          "TailscaleIPs": ["100.64.0.2"], "Online": true, "Active": true, "CurAddr": "203.0.113.5:41641",
          "PublicKey": "nodekey:secret2"},
    "b": {"HostName": "nas", "DNSName": "nas.tail1234.ts.net.", "OS": "linux",
          "TailscaleIPs": ["100.64.0.3"], "Online": true, "Active": true, "Relay": "sin", "ExitNode": true},
    "c": {"HostName": "old-laptop", "DNSName": "old-laptop.tail1234.ts.net.", "OS": "windows",
          "TailscaleIPs": ["100.64.0.4"], "Online": false, "LastSeen": "2026-10-20T08:00:00Z",
          "KeyExpiry": "2027-01-01T00:00:00Z"}
  }
}`

// fakeDaemon serves a canned status on a unix socket, like tailscaled.
func fakeDaemon(t *testing.T, body string, hits *int) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "ts.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("unix sockets unavailable:", err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		if r.URL.Path != "/localapi/v0/status" || r.Host != "local-tailscaled.sock" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return sock
}

func TestDevices(t *testing.T) {
	hits := 0
	c, err := New(fakeDaemon(t, statusJSON, &hits))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	c.now = func() time.Time { return now }

	got, err := c.Devices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Tailnet != "example.github" || got.State != "Running" {
		t.Errorf("tailnet/state = %q/%q", got.Tailnet, got.State)
	}
	var names []string
	for _, d := range got.Devices {
		names = append(names, d.Name)
	}
	// Self first, then online peers by name, then offline.
	if strings.Join(names, ",") != "fedora,nas,pixel-8,old-laptop" {
		t.Fatalf("order = %v", names)
	}
	self, nas, pixel, old := got.Devices[0], got.Devices[1], got.Devices[2], got.Devices[3]
	if !self.Self || !self.Online || self.DNSName != "fedora.tail1234.ts.net" || len(self.IPs) != 2 {
		t.Errorf("self = %+v", self)
	}
	if pixel.Connection != "direct" || nas.Connection != "relay" || !nas.ExitNode {
		t.Errorf("connections: pixel %q, nas %q (exit %v)", pixel.Connection, nas.Connection, nas.ExitNode)
	}
	if old.Online || old.LastSeen == nil || old.KeyExpiry == nil || old.Connection != "" {
		t.Errorf("offline peer = %+v", old)
	}

	// Cached for a few seconds, then fetched again.
	c.Devices(context.Background())
	if hits != 1 {
		t.Errorf("hits = %d, want 1 while cached", hits)
	}
	now = now.Add(cacheFor + time.Second)
	c.Devices(context.Background())
	if hits != 2 {
		t.Errorf("hits = %d, want 2 after the cache expires", hits)
	}
}

func TestDevicesOmitsSecrets(t *testing.T) {
	hits := 0
	c, _ := New(fakeDaemon(t, statusJSON, &hits))
	got, _ := c.Devices(context.Background())
	for _, d := range got.Devices {
		s := strings.Join(append([]string{d.Name, d.DNSName, d.OS, d.Connection}, d.IPs...), " ")
		if strings.Contains(s, "nodekey") || strings.Contains(s, "203.0.113.5") {
			t.Errorf("device leaks a key or public address: %s", s)
		}
	}
}

func TestNewAndErrors(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "missing.sock")); err == nil {
		t.Error("New with a missing socket must fail")
	}
	hits := 0
	c, _ := New(fakeDaemon(t, "not json", &hits))
	if _, err := c.Devices(context.Background()); err == nil {
		t.Error("bad JSON must fail")
	}
	stopped, _ := New(fakeDaemon(t, `{"BackendState":"Stopped","Self":{"HostName":"box"}}`, &hits))
	got, err := stopped.Devices(context.Background())
	if err != nil || got.State != "Stopped" || got.Devices[0].Online {
		t.Errorf("stopped = %+v, %v", got, err)
	}
}

package server

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haelahpx/haepage/internal/tailscale"
)

func TestTailscaleDevices(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "ts.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("unix sockets unavailable:", err)
	}
	up := true
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !up {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"BackendState":"Running","Self":{"HostName":"box","TailscaleIPs":["100.64.0.1"]},"Peer":{"x":{"HostName":"phone","Online":true}}}`))
	})}
	go srv.Serve(ln)
	defer srv.Close()

	c, err := tailscale.New(sock)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Options{Static: built(), Tailscale: c})
	if !strings.Contains(get(t, h, http.MethodGet, "/api/ping").Body.String(), `"tailscale.devices"`) {
		t.Error("ping lacks tailscale.devices")
	}
	rec := get(t, h, http.MethodGet, "/api/tailscale/devices")
	var got tailscale.Devices
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got.Devices) != 2 || !got.Devices[0].Self {
		t.Fatalf("devices = %d %s", rec.Code, rec.Body)
	}

	// Without a client the route doesn't exist.
	if rec := get(t, New(Options{Static: built()}), http.MethodGet, "/api/tailscale/devices"); rec.Code != http.StatusNotFound {
		t.Errorf("no client = %d", rec.Code)
	}
}

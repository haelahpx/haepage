package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

// DefaultSocket is where tailscaled listens on Linux.
const DefaultSocket = "/var/run/tailscale/tailscaled.sock"

// cacheFor keeps status for a few seconds, so many open pages make one
// request to tailscaled.
const cacheFor = 5 * time.Second

// Device is what Haepage shows about one machine on the tailnet. Keys and
// public endpoint addresses are left out.
type Device struct {
	Name       string     `json:"name"`
	DNSName    string     `json:"dnsName,omitempty"`
	OS         string     `json:"os,omitempty"`
	IPs        []string   `json:"ips"`
	Online     bool       `json:"online"`
	LastSeen   *time.Time `json:"lastSeen,omitempty"`
	Self       bool       `json:"self,omitempty"`
	ExitNode   bool       `json:"exitNode,omitempty"`   // traffic goes out through it
	Connection string     `json:"connection,omitempty"` // "direct" or "relay" while active
	KeyExpiry  *time.Time `json:"keyExpiry,omitempty"`
}

// Devices is the response of GET /api/tailscale/devices.
type Devices struct {
	Tailnet string   `json:"tailnet,omitempty"`
	State   string   `json:"state"` // tailscaled's BackendState, e.g. "Running"
	Devices []Device `json:"devices"`
}

// Client reads status from tailscaled's local API.
type Client struct {
	Socket string

	http *http.Client
	now  func() time.Time

	mu      sync.Mutex
	cached  *Devices
	fetched time.Time
}

// New returns a client for socket, or an error when it doesn't exist.
func New(socket string) (*Client, error) {
	if _, err := os.Stat(socket); err != nil {
		return nil, fmt.Errorf("no tailscaled socket at %s", socket)
	}
	d := net.Dialer{Timeout: 2 * time.Second}
	return &Client{
		Socket: socket,
		http: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return d.DialContext(ctx, "unix", socket)
				},
			},
		},
		now: time.Now,
	}, nil
}

// status is the part of /localapi/v0/status that Haepage reads.
type status struct {
	BackendState   string
	CurrentTailnet *struct{ Name string }
	Self           *peer
	Peer           map[string]*peer
}

type peer struct {
	HostName     string
	DNSName      string
	OS           string
	TailscaleIPs []string
	Online       bool
	Active       bool
	ExitNode     bool
	CurAddr      string
	Relay        string
	LastSeen     time.Time
	KeyExpiry    *time.Time
}

// Devices returns this machine and its peers: online first, then by name.
func (c *Client) Devices(ctx context.Context) (Devices, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil && c.now().Sub(c.fetched) < cacheFor {
		return *c.cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://local-tailscaled.sock/localapi/v0/status", nil)
	if err != nil {
		return Devices{}, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return Devices{}, fmt.Errorf("asking tailscaled: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return Devices{}, fmt.Errorf("tailscaled: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var st status
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&st); err != nil {
		return Devices{}, fmt.Errorf("reading tailscaled status: %w", err)
	}

	out := Devices{State: st.BackendState, Devices: []Device{}}
	if st.CurrentTailnet != nil {
		out.Tailnet = st.CurrentTailnet.Name
	}
	if st.Self != nil {
		d := device(st.Self)
		d.Self, d.Online, d.Connection, d.LastSeen = true, st.BackendState == "Running", "", nil
		out.Devices = append(out.Devices, d)
	}
	for _, p := range st.Peer {
		out.Devices = append(out.Devices, device(p))
	}
	slices.SortStableFunc(out.Devices, func(a, b Device) int {
		switch {
		case a.Self != b.Self:
			return boolOrder(a.Self)
		case a.Online != b.Online:
			return boolOrder(a.Online)
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	c.cached, c.fetched = &out, c.now()
	return out, nil
}

func boolOrder(first bool) int {
	if first {
		return -1
	}
	return 1
}

func device(p *peer) Device {
	d := Device{
		Name:     p.HostName,
		DNSName:  strings.TrimSuffix(p.DNSName, "."),
		OS:       p.OS,
		IPs:      p.TailscaleIPs,
		Online:   p.Online,
		ExitNode: p.ExitNode,
	}
	if d.IPs == nil {
		d.IPs = []string{}
	}
	if label, _, ok := strings.Cut(d.DNSName, "."); ok && label != "" {
		d.Name = label // the MagicDNS name is what people type
	}
	if !p.Online && !p.LastSeen.IsZero() {
		t := p.LastSeen
		d.LastSeen = &t
	}
	if p.KeyExpiry != nil && !p.KeyExpiry.IsZero() {
		d.KeyExpiry = p.KeyExpiry
	}
	if p.Active {
		if p.CurAddr != "" {
			d.Connection = "direct"
		} else if p.Relay != "" {
			d.Connection = "relay"
		}
	}
	return d
}

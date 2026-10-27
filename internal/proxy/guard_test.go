package proxy

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

func TestCheck(t *testing.T) {
	g := &Guard{AllowPrivate: func(host string) bool { return host == "nas.lan" }}
	cases := []struct {
		host, ip string
		ok       bool
	}{
		{"example.com", "93.184.216.34", true},
		{"example.com", "2606:2800:220:1::1", true},
		{"tailnet", "100.101.102.103", true},
		{"x", "127.0.0.1", false},
		{"x", "::1", false},
		{"x", "::ffff:127.0.0.1", false},
		{"x", "0.0.0.0", false},
		{"x", "0.1.2.3", false},
		{"x", "169.254.169.254", false},
		{"x", "fe80::1", false},
		{"x", "224.0.0.1", false},
		{"x", "192.168.1.10", false},
		{"x", "10.0.0.1", false},
		{"x", "172.17.0.1", false},
		{"x", "fd00::1", false},
		{"nas.lan", "192.168.1.10", true},
		{"nas.lan", "127.0.0.1", false},
	}
	for _, c := range cases {
		err := g.Check(c.host, netip.MustParseAddr(c.ip))
		if (err == nil) != c.ok {
			t.Errorf("Check(%s, %s) = %v, want ok=%v", c.host, c.ip, err, c.ok)
		}
		if err != nil && !errors.Is(err, ErrBlocked) {
			t.Errorf("Check(%s, %s) error %v is not ErrBlocked", c.host, c.ip, err)
		}
	}
	if (&Guard{}).Check("x", netip.MustParseAddr("192.168.1.1")) == nil {
		t.Error("nil AllowPrivate must block private addresses")
	}
}

type fakeResolver map[string][]netip.Addr

func (f fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if ips, ok := f[host]; ok {
		return ips, nil
	}
	return nil, errors.New("no such host")
}

func TestDialContextDialsTheCheckedAddress(t *testing.T) {
	var dialed []string
	g := &Guard{
		Resolver: fakeResolver{
			"mixed.example": {netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("93.184.216.34")},
			"local.example": {netip.MustParseAddr("10.0.0.5")},
		},
		dial: func(_ context.Context, _, addr string) (net.Conn, error) {
			dialed = append(dialed, addr)
			c, _ := net.Pipe()
			return c, nil
		},
	}
	conn, err := g.DialContext(context.Background(), "tcp", "mixed.example:443")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if len(dialed) != 1 || dialed[0] != "93.184.216.34:443" {
		t.Fatalf("dialed %v, want only the public address", dialed)
	}

	dialed = nil
	if _, err := g.DialContext(context.Background(), "tcp", "local.example:443"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("private-only host: err = %v, want ErrBlocked", err)
	}
	if _, err := g.DialContext(context.Background(), "tcp", "[::1]:443"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("literal loopback: err = %v, want ErrBlocked", err)
	}
	if len(dialed) != 0 {
		t.Fatalf("blocked hosts were dialed: %v", dialed)
	}
}

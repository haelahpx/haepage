package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// ErrBlocked means the address is one a remote URL must not reach.
var ErrBlocked = errors.New("address not allowed")

// Guard stops server-side fetches from reaching this machine or the local
// network (IMPLEMENTATION.md §7.7). It resolves the host itself, checks
// every address, and dials the address it checked, so a DNS answer can't
// change between the check and the connection.
type Guard struct {
	// AllowPrivate reports whether a private address may be used for this
	// host name (a service in the config, or settings.proxyAllow). Nil
	// allows none.
	AllowPrivate func(host string) bool
	// Resolver looks up host names. Nil uses net.DefaultResolver.
	Resolver interface {
		LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
	}
	dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

var zeroNet = netip.MustParsePrefix("0.0.0.0/8")

// Check reports whether host may be reached at ip.
func (g *Guard) Check(host string, ip netip.Addr) error {
	ip = ip.Unmap()
	switch {
	case !ip.IsValid(), ip.IsUnspecified(), zeroNet.Contains(ip),
		ip.IsLoopback(), ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast(),
		ip.IsInterfaceLocalMulticast(), ip.IsMulticast():
		return fmt.Errorf("%w: %s is a local address", ErrBlocked, host)
	case ip.IsPrivate() && (g.AllowPrivate == nil || !g.AllowPrivate(host)):
		return fmt.Errorf("%w: %s is on a private network (add it to settings.proxyAllow to allow it)", ErrBlocked, host)
	}
	return nil
}

// DialContext resolves addr, checks each address and dials the first one
// that passes and answers.
func (g *Guard) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	var ips []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		ips = []netip.Addr{ip}
	} else {
		var r interface {
			LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
		} = net.DefaultResolver
		if g.Resolver != nil {
			r = g.Resolver
		}
		if ips, err = r.LookupNetIP(ctx, "ip", host); err != nil {
			return nil, err
		}
	}
	dial := g.dial
	if dial == nil {
		dial = (&net.Dialer{Timeout: 5 * time.Second}).DialContext
	}
	var lastErr error = fmt.Errorf("%w: no addresses for %s", ErrBlocked, host)
	for _, ip := range ips {
		if err := g.Check(host, ip); err != nil {
			lastErr = err
			continue
		}
		conn, err := dial(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// Client returns an HTTP client that dials through the guard, ignores
// proxy environment variables (a proxy would dial for us), and follows at
// most three redirects, each dialled through the guard again.
func (g *Guard) Client(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           g.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: timeout,
			MaxIdleConns:          4,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" {
				return errors.New("redirect away from https")
			}
			return nil
		},
	}
}

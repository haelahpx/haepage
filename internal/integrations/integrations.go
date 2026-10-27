// Package integrations adds live data from apps Haepage detects: what's
// playing on a media server, pending requests, download queues. API keys
// stay on the server (secrets.json); the browser only sees the results.
package integrations

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/store"
)

// Conn is how to reach one app.
type Conn struct {
	BaseURL string
	Key     string
}

// Activity is one thing that happened in an app, shown in the activity
// feed and as a notification.
type Activity struct {
	ID      string    `json:"id"`
	Service string    `json:"service"`
	Kind    string    `json:"kind"`
	Type    string    `json:"type"` // playback.start, request.new, request.approved, request.available, download.grabbed, download.imported, health.warning, test
	Title   string    `json:"title"`
	Detail  string    `json:"detail,omitempty"`
	Image   string    `json:"image,omitempty"`
	At      time.Time `json:"at"`
}

// State is what the browser receives for a connected integration.
type State struct {
	Service string    `json:"service"`
	Kind    string    `json:"kind"`
	Status  string    `json:"status"` // ok, error, waiting
	Error   string    `json:"error,omitempty"`
	Data    any       `json:"data,omitempty"`
	Updated time.Time `json:"updated"`
}

// Integration is the support for one app.
type Integration interface {
	// Test checks the address and key, returning a message fit for users.
	Test(ctx context.Context, c Conn) error
	// Poll returns the current snapshot.
	Poll(ctx context.Context, c Conn) (any, error)
	// Diff turns two snapshots into activity (prev is never nil).
	Diff(prev, cur any) []Activity
	// Webhook parses a webhook body from the app (nil if not supported).
	Webhook(body []byte) []Activity
	// Interval is how often to poll while someone has the page open.
	Interval() time.Duration
}

// Kinds lists the built-in integrations.
func Kinds(client *http.Client) map[string]Integration {
	h := httpClient{client}
	return map[string]Integration{
		"jellyfin": &Jellyfin{http: h},
		"seerr":    &Seerr{http: h},
		"radarr":   &Arr{http: h, App: "radarr"},
		"sonarr":   &Arr{http: h, App: "sonarr"},
	}
}

var (
	// ErrUnauthorized means the app rejected the API key.
	ErrUnauthorized = errors.New("the app rejected the API key")
	ErrNotFound     = errors.New("unknown service")
	ErrNoSupport    = errors.New("this service has no integration")
)

const (
	maxBody    = 2 << 20
	reqTimeout = 8 * time.Second
	maxBackoff = 5 * time.Minute
)

// httpClient wraps calls to an app's API.
type httpClient struct{ c *http.Client }

func (h httpClient) get(ctx context.Context, url string, header http.Header) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, reqTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("bad address: %w", err)
	}
	req.Header = header
	req.Header.Set("Accept", "application/json")
	res, err := h.c.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("could not reach the app")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return nil, "", fmt.Errorf("could not read the reply")
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return nil, "", ErrUnauthorized
	case res.StatusCode >= 300:
		return nil, "", fmt.Errorf("the app answered HTTP %d", res.StatusCode)
	}
	return body, res.Header.Get("Content-Type"), nil
}

func (h httpClient) getJSON(ctx context.Context, url string, header http.Header, out any) error {
	body, _, err := h.get(ctx, url, header)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("unexpected reply (is this the right app?)")
	}
	return nil
}

// conn is the manager's record of one connected service.
type conn struct {
	kind     string
	prev     any
	next     time.Time
	failures int
	hooked   bool // a webhook arrived: it reports events, so polls don't
}

// Manager runs the connected integrations.
type Manager struct {
	Registry  *store.Registry
	Secrets   *config.Secrets
	CheckHost string // replaces "{host}" in service URLs
	Kinds     map[string]Integration

	now   func() time.Time
	mu    sync.Mutex
	conns map[string]*conn
	seq   int
}

func keyName(service string) string  { return "int:" + service }
func hookName(service string) string { return "hook:" + service }

func (m *Manager) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

// target finds a service and how to reach it.
func (m *Manager) target(service string) (store.Service, Integration, string, error) {
	svc, ok := m.Registry.Get(service)
	if !ok {
		return svc, nil, "", ErrNotFound
	}
	in, ok := m.Kinds[svc.Integration]
	if !ok {
		return svc, nil, "", ErrNoSupport
	}
	base := svc.URLs.LAN
	if base == "" {
		base = svc.URLs.Public
	}
	if base == "" {
		base = svc.URLs.Tailscale
	}
	if base == "" {
		return svc, nil, "", fmt.Errorf("the service has no address")
	}
	host := m.CheckHost
	if host == "" {
		host = "127.0.0.1"
	}
	return svc, in, strings.TrimRight(strings.ReplaceAll(base, "{host}", host), "/"), nil
}

// Connect tests a key and, if it works, saves it and starts polling.
func (m *Manager) Connect(ctx context.Context, service, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("enter an API key")
	}
	svc, in, base, err := m.target(service)
	if err != nil {
		return err
	}
	if err := in.Test(ctx, Conn{BaseURL: base, Key: key}); err != nil {
		return err
	}
	if err := m.Secrets.Set(keyName(service), key); err != nil {
		return fmt.Errorf("could not save the key: %w", err)
	}
	if m.Secrets.Get(hookName(service)) == "" {
		if err := m.Secrets.Set(hookName(service), newToken()); err != nil {
			return fmt.Errorf("could not save the webhook token: %w", err)
		}
	}
	m.mu.Lock()
	if m.conns == nil {
		m.conns = map[string]*conn{}
	}
	m.conns[service] = &conn{kind: svc.Integration}
	m.mu.Unlock()
	m.Registry.SetIntegration(service, State{Service: service, Kind: svc.Integration, Status: "waiting", Updated: m.clock()})
	return nil
}

// Disconnect forgets the key and webhook token.
func (m *Manager) Disconnect(service string) error {
	if err := m.Secrets.Set(keyName(service), ""); err != nil {
		return err
	}
	if err := m.Secrets.Set(hookName(service), ""); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.conns, service)
	m.mu.Unlock()
	m.Registry.SetIntegration(service, nil)
	return nil
}

// Info describes one service that can be connected.
type Info struct {
	Service   string `json:"service"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Connected bool   `json:"connected"`
	Webhook   string `json:"webhook,omitempty"` // path, e.g. /api/hooks/<token>
}

// List returns every service with an integration, connected or not.
func (m *Manager) List() []Info {
	var out []Info
	for _, svc := range m.Registry.Snapshot() {
		if _, ok := m.Kinds[svc.Integration]; !ok {
			continue
		}
		info := Info{Service: svc.ID, Name: svc.Name, Kind: svc.Integration, Connected: m.Secrets.Get(keyName(svc.ID)) != ""}
		if token := m.Secrets.Get(hookName(svc.ID)); info.Connected && token != "" {
			info.Webhook = "/api/hooks/" + token
		}
		out = append(out, info)
	}
	return out
}

// Webhook handles an incoming webhook. It reports false for an unknown
// token. Tokens are compared in constant time.
func (m *Manager) Webhook(token string, body []byte) bool {
	var service string
	for _, svc := range m.Registry.Snapshot() {
		want := m.Secrets.Get(hookName(svc.ID))
		if want != "" && subtle.ConstantTimeCompare([]byte(want), []byte(token)) == 1 {
			service = svc.ID
		}
	}
	if service == "" {
		return false
	}
	svc, in, _, err := m.target(service)
	if err != nil {
		return false
	}
	for _, a := range in.Webhook(body) {
		m.emit(svc.ID, svc.Integration, a)
	}
	m.mu.Lock()
	if c, ok := m.conns[service]; ok {
		c.hooked = true
		c.next = time.Time{} // refresh the tiles now
	}
	m.mu.Unlock()
	return true
}

func (m *Manager) emit(service, kind string, a Activity) {
	m.mu.Lock()
	m.seq++
	a.ID = fmt.Sprintf("%d-%d", m.clock().UnixMilli(), m.seq)
	m.mu.Unlock()
	a.Service, a.Kind = service, kind
	if a.At.IsZero() {
		a.At = m.clock()
	}
	m.Registry.AddActivity(a)
}

// Run polls connected integrations until ctx ends. Nothing is polled while
// nobody has the page open; webhooks still arrive.
func (m *Manager) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		m.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Tick does one round: picks up keys saved earlier, then polls what's due.
func (m *Manager) Tick(ctx context.Context) {
	m.mu.Lock()
	if m.conns == nil {
		m.conns = map[string]*conn{}
	}
	m.mu.Unlock()
	for _, svc := range m.Registry.Snapshot() {
		if _, ok := m.Kinds[svc.Integration]; !ok || m.Secrets.Get(keyName(svc.ID)) == "" {
			continue
		}
		m.mu.Lock()
		if _, ok := m.conns[svc.ID]; !ok {
			m.conns[svc.ID] = &conn{kind: svc.Integration}
		}
		m.mu.Unlock()
	}
	if m.Registry.Subscribers() == 0 {
		return
	}
	now := m.clock()
	m.mu.Lock()
	var due []string
	for id, c := range m.conns {
		if !now.Before(c.next) {
			due = append(due, id)
		}
	}
	m.mu.Unlock()
	for _, id := range due {
		m.poll(ctx, id)
	}
}

func (m *Manager) poll(ctx context.Context, service string) {
	svc, in, base, err := m.target(service)
	m.mu.Lock()
	c := m.conns[service]
	m.mu.Unlock()
	if c == nil {
		return
	}
	if err != nil {
		m.mu.Lock()
		delete(m.conns, service)
		m.mu.Unlock()
		return
	}
	data, err := in.Poll(ctx, Conn{BaseURL: base, Key: m.Secrets.Get(keyName(service))})
	now := m.clock()
	if err != nil {
		m.mu.Lock()
		c.failures++
		backoff := min(10*time.Second<<min(c.failures-1, 5), maxBackoff)
		c.next = now.Add(backoff)
		m.mu.Unlock()
		m.Registry.SetIntegration(service, State{Service: service, Kind: svc.Integration, Status: "error", Error: err.Error(), Updated: now})
		return
	}
	m.mu.Lock()
	prev, hooked := c.prev, c.hooked
	c.prev, c.failures, c.next = data, 0, now.Add(in.Interval())
	m.mu.Unlock()
	if prev != nil && !hooked {
		for _, a := range in.Diff(prev, data) {
			m.emit(service, svc.Integration, a)
		}
	}
	m.Registry.SetIntegration(service, State{Service: service, Kind: svc.Integration, Status: "ok", Data: data, Updated: now})
}

// Image fetches an image from an app (posters), for integrations that
// serve them only with the API key.
func (m *Manager) Image(ctx context.Context, service, item string) ([]byte, string, error) {
	_, in, base, err := m.target(service)
	if err != nil {
		return nil, "", err
	}
	img, ok := in.(interface {
		Image(ctx context.Context, c Conn, item string) ([]byte, string, error)
	})
	if !ok {
		return nil, "", ErrNoSupport
	}
	return img.Image(ctx, Conn{BaseURL: base, Key: m.Secrets.Get(keyName(service))}, item)
}

func newToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

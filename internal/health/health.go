// Package health checks services on a schedule and reports up or down.
package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/haelahpx/haepage/internal/store"
)

// Target is one thing to check.
type Target struct {
	ID       string
	Type     string // http, tcp or none
	URL      string
	Expect   int // exact HTTP status; 0 means any status below 500
	Interval time.Duration
	Insecure bool  // skip TLS verification
	Running  *bool // container state; false is reported as down without a request
}

const (
	defaultInterval = 60 * time.Second
	retryDown       = 15 * time.Second
	httpTimeout     = 5 * time.Second
	tcpTimeout      = 3 * time.Second
)

// Scheduler runs checks with a small worker pool. Each target repeats on
// its own interval with ±10 % jitter, so checks don't all fire together; a
// target that goes down is checked again sooner.
type Scheduler struct {
	Targets func() []Target
	Report  func(id string, h store.Health)
	Workers int // default 4

	now     func() time.Time
	mu      sync.Mutex
	next    map[string]time.Time
	busy    map[string]bool
	running map[string]bool // last known container state, to re-check on change
}

func (s *Scheduler) Run(ctx context.Context) {
	if s.Workers == 0 {
		s.Workers = 4
	}
	if s.now == nil {
		s.now = time.Now
	}
	s.next, s.busy, s.running = map[string]time.Time{}, map[string]bool{}, map[string]bool{}

	jobs := make(chan Target, s.Workers)
	var wg sync.WaitGroup
	for range s.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range jobs {
				h := Check(ctx, t)
				s.Report(t.ID, h)
				s.done(t, h)
			}
		}()
	}

	tick := time.NewTicker(time.Second)
	defer func() {
		tick.Stop()
		close(jobs)
		wg.Wait()
	}()
	for {
		s.dispatch(ctx, jobs)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// dispatch queues every due target that isn't already being checked.
func (s *Scheduler) dispatch(ctx context.Context, jobs chan<- Target) {
	now := s.now()
	targets := s.Targets()
	live := make(map[string]bool, len(targets))
	for _, t := range targets {
		live[t.ID] = true
		if t.Type == "none" {
			continue
		}
		s.mu.Lock()
		if t.Running != nil {
			// A container that just started or stopped is checked now.
			if was, seen := s.running[t.ID]; seen && was != *t.Running {
				s.next[t.ID] = now
			}
			s.running[t.ID] = *t.Running
		}
		due, known := s.next[t.ID]
		if !known {
			// Spread first checks over a couple of seconds.
			due = now.Add(time.Duration(rand.Int64N(int64(2 * time.Second))))
			s.next[t.ID] = due
		}
		ready := !s.busy[t.ID] && !now.Before(due)
		if ready {
			s.busy[t.ID] = true
		}
		s.mu.Unlock()
		if !ready {
			continue
		}
		select {
		case jobs <- t:
		case <-ctx.Done():
			return
		default: // all workers busy; try again next tick
			s.mu.Lock()
			s.busy[t.ID] = false
			s.mu.Unlock()
		}
	}
	s.mu.Lock()
	for id := range s.next {
		if !live[id] {
			delete(s.next, id)
			delete(s.running, id)
		}
	}
	s.mu.Unlock()
}

func (s *Scheduler) done(t Target, h store.Health) {
	interval := t.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	if h.Status == "down" && interval > retryDown {
		interval = retryDown
	}
	jitter := time.Duration(rand.Int64N(int64(interval)/5+1)) - interval/10
	s.mu.Lock()
	s.next[t.ID] = s.now().Add(interval + jitter)
	s.busy[t.ID] = false
	s.mu.Unlock()
}

var (
	plainClient    = newClient(false)
	insecureClient = newClient(true)
)

// newClient doesn't follow redirects: a redirect means the service is up.
func newClient(insecure bool) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = 1
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in per service
	}
	return &http.Client{
		Transport:     tr,
		Timeout:       httpTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Check runs one check now.
func Check(ctx context.Context, t Target) store.Health {
	if t.Running != nil && !*t.Running {
		return store.Health{Status: "down", Reason: store.StoppedReason}
	}
	start := time.Now()
	var err error
	switch t.Type {
	case "tcp":
		err = checkTCP(ctx, t.URL)
	default:
		err = checkHTTP(ctx, t)
	}
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return store.Health{Status: "down", MS: ms, Reason: err.Error()}
	}
	return store.Health{Status: "up", MS: ms}
}

func checkHTTP(ctx context.Context, t Target) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return fmt.Errorf("bad URL")
	}
	req.Header.Set("User-Agent", "Haepage health check")
	client := plainClient
	if t.Insecure {
		client = insecureClient
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("no response")
	}
	io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
	res.Body.Close()
	// Any answer below 500 means the service is running, even a login
	// page (401) or a missing path (404), unless an exact status is set.
	if t.Expect != 0 && res.StatusCode != t.Expect {
		return fmt.Errorf("HTTP %d, expected %d", res.StatusCode, t.Expect)
	}
	if t.Expect == 0 && res.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return nil
}

// checkTCP dials host:port taken from a URL or a bare address.
func checkTCP(ctx context.Context, target string) error {
	addr := target
	if u, err := url.Parse(target); err == nil && u.Host != "" {
		addr = u.Host
		if u.Port() == "" {
			port := "80"
			if u.Scheme == "https" {
				port = "443"
			}
			addr = net.JoinHostPort(u.Hostname(), port)
		}
	}
	d := net.Dialer{Timeout: tcpTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connection refused or timed out")
	}
	return conn.Close()
}

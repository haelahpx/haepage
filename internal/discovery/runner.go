package discovery

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Runner keeps the detected list current: Docker through its event stream
// (re-listing after each burst of events), listening ports on a timer.
type Runner struct {
	Docker    *DockerClient // nil turns Docker discovery off
	Ports     *PortScanner  // nil turns port discovery off
	Apps      *KnownApps
	OwnPort   int           // Haepage's own port, never reported
	PortEvery time.Duration // default 15s
	OnChange  func([]Detected)
	Logger    *slog.Logger

	mu        sync.Mutex
	docker    []Detected
	dockerPub map[int]bool
	ports     []Detected
}

func (r *Runner) Run(ctx context.Context) {
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
	if r.PortEvery == 0 {
		r.PortEvery = 15 * time.Second
	}
	if r.Ports != nil && r.Ports.Apps == nil {
		r.Ports.Apps = r.Apps
	}
	var wg sync.WaitGroup
	if r.Docker != nil {
		wg.Add(1)
		go func() { defer wg.Done(); r.watchDocker(ctx) }()
	}
	if r.Ports != nil {
		wg.Add(1)
		go func() { defer wg.Done(); r.watchPorts(ctx) }()
	}
	wg.Wait()
}

func (r *Runner) publish() {
	r.mu.Lock()
	all := append([]Detected{}, r.docker...)
	// The port scan may have run before the latest container list, so
	// ports Docker publishes are dropped here too.
	for _, d := range r.ports {
		if !r.dockerPub[d.Port] {
			all = append(all, d)
		}
	}
	r.mu.Unlock()
	if r.OnChange != nil {
		r.OnChange(all)
	}
}

func (r *Runner) relistDocker(ctx context.Context) error {
	list, err := r.Docker.Containers(ctx)
	if err != nil {
		return err
	}
	var found []Detected
	published := map[int]bool{}
	for _, c := range list {
		c = r.Docker.withConfiguredPorts(ctx, c)
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				published[p.PublicPort] = true
			}
		}
		if d, ok := FromContainer(c, r.Apps); ok {
			found = append(found, d)
		}
	}
	r.mu.Lock()
	r.docker, r.dockerPub = found, published
	r.mu.Unlock()
	r.publish()
	return nil
}

// watchDocker lists containers, then follows the event stream. Events
// arrive in bursts (a compose up starts many containers), so a re-list
// waits for a short quiet period. On any error it backs off from 1s to 30s
// and lists again, so missed events can't leave stale state.
func (r *Runner) watchDocker(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		if err := r.relistDocker(ctx); err != nil {
			r.Logger.Warn("docker discovery", "err", err, "retry", backoff)
			if !sleep(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second

		events := make(chan struct{}, 1)
		streamCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			done <- r.Docker.Events(streamCtx, func(string) {
				select {
				case events <- struct{}{}:
				default:
				}
			})
		}()

	follow:
		for {
			select {
			case <-ctx.Done():
				cancel()
				return
			case err := <-done:
				r.Logger.Warn("docker event stream ended", "err", err)
				break follow
			case <-events:
				sleep(ctx, 500*time.Millisecond) // let the burst finish
				if err := r.relistDocker(ctx); err != nil {
					r.Logger.Warn("docker discovery", "err", err)
				}
			}
		}
		cancel()
		if !sleep(ctx, backoff) {
			return
		}
	}
}

func (r *Runner) watchPorts(ctx context.Context) {
	t := time.NewTicker(r.PortEvery)
	defer t.Stop()
	for {
		r.mu.Lock()
		skip := map[int]bool{r.OwnPort: true}
		for p := range r.dockerPub {
			skip[p] = true
		}
		r.mu.Unlock()
		found := r.Ports.Scan(ctx, skip)
		r.mu.Lock()
		changed := !sameKeys(found, r.ports)
		r.ports = found
		r.mu.Unlock()
		if changed {
			r.publish()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func sameKeys(a, b []Detected) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key || a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// LocalHosts returns a check for "is this host name this machine": its
// addresses, its host name, and localhost.
func LocalHosts() func(string) bool {
	set := map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true, "0.0.0.0": true}
	if name, err := os.Hostname(); err == nil {
		set[strings.ToLower(name)] = true
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				set[ipnet.IP.String()] = true
			}
		}
	}
	return func(host string) bool { return set[strings.ToLower(host)] }
}

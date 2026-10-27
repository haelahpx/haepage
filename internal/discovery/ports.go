package discovery

import (
	"bufio"
	"context"
	"encoding/hex"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/haelahpx/haepage/internal/store"
)

// skipPorts are well-known ports that never serve a web UI.
var skipPorts = map[int]bool{
	22: true, 25: true, 53: true, 111: true, 139: true, 445: true, 631: true, 993: true, 995: true,
	1883: true, 3306: true, 5353: true, 5432: true, 6379: true, 11211: true, 27017: true,
}

// ListeningPorts returns TCP ports listening on a non-loopback address,
// read from /proc/net/tcp and /proc/net/tcp6 under root. This needs no
// special permissions.
func ListeningPorts(root string) []int {
	set := map[int]bool{}
	for _, name := range []string{"proc/net/tcp", "proc/net/tcp6"} {
		f, err := os.Open(filepath.Join(root, name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Scan() // header
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 4 || fields[3] != "0A" { // 0A = LISTEN
				continue
			}
			addr, portHex, ok := strings.Cut(fields[1], ":")
			if !ok || loopback(addr) {
				continue
			}
			if p, err := strconv.ParseUint(portHex, 16, 16); err == nil {
				set[int(p)] = true
			}
		}
		f.Close()
	}
	ports := make([]int, 0, len(set))
	for p := range set {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

// loopback reports whether a /proc/net address (hex, little-endian per
// 32-bit word) is 127.0.0.0/8, ::1 or an IPv4-mapped loopback.
func loopback(addr string) bool {
	b, err := hex.DecodeString(addr)
	if err != nil {
		return false
	}
	switch len(b) {
	case 4:
		return b[3] == 127
	case 16:
		if strings.EqualFold(addr, "00000000000000000000000001000000") {
			return true
		}
		// ::ffff:127.x.x.x is stored as 0000 0000 FFFF0000 xx00007F
		return strings.EqualFold(addr[:24], "0000000000000000FFFF0000") && b[15] == 127
	}
	return false
}

var titleRE = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// PortScanner finds web servers on listening ports that Docker doesn't
// already explain. Each new port gets one quick HTTP request; ports that
// don't answer HTTP are remembered and skipped.
type PortScanner struct {
	Root   string     // "/" in production
	Apps   *KnownApps // names ports that only one known app uses
	Probe  func(ctx context.Context, port int) (title string, ok bool)
	mu     sync.Mutex
	cache  map[int]probeResult
	maxAge time.Duration
}

type probeResult struct {
	web   bool
	title string
	at    time.Time
}

func NewPortScanner() *PortScanner {
	return &PortScanner{Root: "/", Probe: httpProbe}
}

// Scan returns detected services for listening ports not in skip.
func (s *PortScanner) Scan(ctx context.Context, skip map[int]bool) []Detected {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		s.cache = map[int]probeResult{}
	}
	if s.maxAge == 0 {
		s.maxAge = 10 * time.Minute
	}
	var out []Detected
	for _, port := range ListeningPorts(s.Root) {
		if skip[port] || skipPorts[port] {
			continue
		}
		r, ok := s.cache[port]
		if !ok || time.Since(r.at) > s.maxAge {
			title, web := s.Probe(ctx, port)
			r = probeResult{web: web, title: title, at: time.Now()}
			s.cache[port] = r
		}
		if !r.web {
			continue
		}
		p := strconv.Itoa(port)
		d := Detected{Name: firstNonEmpty(r.title, "Port "+p)}
		if app := s.Apps.ByUniquePort(port); app != nil {
			d = Detected{Name: app.Name, Icon: app.Icon, Group: app.Group, Integration: app.Integration}
		}
		out = append(out, Detected{
			Key:         "port:" + p,
			Source:      "port",
			Name:        d.Name,
			Icon:        d.Icon,
			Group:       d.Group,
			Integration: d.Integration,
			URLs:        store.URLs{LAN: schemeFor(port) + "://{host}:" + p},
			Port:        port,
			CheckType:   "http",
			CheckURL:    schemeFor(port) + "://127.0.0.1:" + p,
		})
	}
	return out
}

// httpProbe asks the port for "/" and reads the page title, if any.
func httpProbe(ctx context.Context, port int) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/", nil)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	title := ""
	if m := titleRE.FindSubmatch(body); m != nil {
		title = strings.Join(strings.Fields(html.UnescapeString(string(m[1]))), " ")
		if len(title) > 60 {
			title = title[:60]
		}
	}
	return title, true
}

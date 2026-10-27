package discovery

import (
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/store"
)

// InboxGroup holds newly detected services until the user accepts them.
const InboxGroup = "inbox"

// Merge combines the user's services with detected ones (vision §16):
//
//   - Manual services always come first, in config order.
//   - A detected service on the same local port as a manual one is dropped.
//   - Field precedence: the user's override, then labels and the known-apps
//     match (already applied in Detected), then inferred defaults.
//   - New detected services go to the Inbox unless settings.inbox is "auto".
//   - Detection never writes the config; only user actions do.
func Merge(cfg config.Config, detected []Detected, isLocal func(host string) bool) []store.Service {
	out := make([]store.Service, 0, len(cfg.Services)+len(detected))
	manualPorts := map[int]bool{}

	for _, m := range cfg.Services {
		s := store.Service{
			ID:          m.ID,
			Source:      "manual",
			State:       "pinned",
			Name:        m.Name,
			Icon:        m.Icon,
			Group:       m.Group,
			Description: m.Description,
			URLs:        store.URLs(m.URLs),
			Integration: m.Integration,
		}
		applyCheck(&s, m.Check, firstNonEmpty(m.URLs.LAN, m.URLs.Public, m.URLs.Tailscale))
		for _, u := range []string{m.URLs.LAN, m.URLs.Tailscale, m.URLs.Public} {
			if host, port := hostPort(u); port > 0 && (host == "{host}" || isLocal(host)) {
				manualPorts[port] = true
			}
		}
		out = append(out, s)
	}

	auto := cfg.Settings.Inbox == "auto"
	var found []store.Service
	for _, d := range detected {
		if d.Port > 0 && manualPorts[d.Port] {
			continue
		}
		o, hasOverride := cfg.Overrides[d.Key]
		s := store.Service{
			ID:          d.Key,
			Source:      d.Source,
			Name:        firstNonEmpty(o.Name, d.Name),
			Icon:        firstNonEmpty(o.Icon, d.Icon),
			Suggested:   d.Group,
			Description: d.Description,
			URLs:        d.URLs,
			Running:     d.Running,
			Integration: d.Integration,
		}
		if o.URLs != nil {
			s.URLs = store.URLs(*o.URLs)
		}
		switch {
		case o.Hidden:
			s.State = "hidden"
		case o.Pinned:
			s.State = "pinned"
			s.Group = firstNonEmpty(o.Group, d.Group)
		default:
			s.State = "detected"
			s.IsNew = !hasOverride
			if auto && d.Group != "" {
				s.Group = d.Group
			} else {
				s.Group = InboxGroup
			}
		}
		s.CheckType, s.CheckURL = d.CheckType, d.CheckURL
		if d.Running != nil && !*d.Running {
			s.Health = &store.Health{Status: "down", Reason: store.StoppedReason}
		}
		found = append(found, s)
	}
	sort.SliceStable(found, func(i, j int) bool { return strings.ToLower(found[i].Name) < strings.ToLower(found[j].Name) })
	out = append(out, found...)
	for i := range out {
		out[i].NoCheck = out[i].CheckType == "none"
	}
	return out
}

func applyCheck(s *store.Service, c *config.Check, target string) {
	s.CheckType = "http"
	if c != nil {
		if c.Type != "" {
			s.CheckType = c.Type
		}
		s.CheckExpect, s.CheckInterval, s.CheckInsecure = c.Expect, c.Interval, c.Insecure
		if c.Path != "" && target != "" {
			target = strings.TrimRight(target, "/") + "/" + strings.TrimLeft(c.Path, "/")
		}
	}
	s.CheckURL = target
}

// hostPort extracts host and port from a URL, using the scheme's default
// port when none is given.
func hostPort(raw string) (string, int) {
	if raw == "" {
		return "", 0
	}
	u, err := url.Parse(strings.ReplaceAll(raw, "{host}", "haepage-host-placeholder"))
	if err != nil {
		return "", 0
	}
	host := u.Hostname()
	if host == "haepage-host-placeholder" {
		host = "{host}"
	}
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		switch u.Scheme {
		case "http":
			port = 80
		case "https":
			port = 443
		}
	}
	return host, port
}

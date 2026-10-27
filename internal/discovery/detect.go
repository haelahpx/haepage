package discovery

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/haelahpx/haepage/internal/store"
)

// Detected is a service found by discovery, before it is merged with the
// user's config.
type Detected struct {
	Key         string // stable identity, e.g. "docker:media/app" or "port:8080"
	Source      string // docker or port
	Name        string
	Icon        string
	Group       string // suggested group
	Description string
	URLs        store.URLs
	Port        int // published host port, 0 if unknown
	Running     *bool
	Integration string
	Public      bool

	CheckType string // http, tcp or none
	CheckURL  string // what the server checks
}

// label reads haepage.<name>, falling back to the dash.<name> alias.
func label(labels map[string]string, name string) string {
	if v, ok := labels["haepage."+name]; ok {
		return v
	}
	return labels["dash."+name]
}

// FromContainer turns a container into a Detected service. ok is false for
// containers that should not appear: opted out, or no reachable web port
// and no URL label.
func FromContainer(c Container, apps *KnownApps) (Detected, bool) {
	l := c.Labels
	if strings.EqualFold(label(l, "enable"), "false") || isHaepage(c) {
		return Detected{}, false
	}
	app := apps.Match(c.Image)

	d := Detected{Key: containerKey(c), Source: "docker"}
	running := c.State == "running"
	d.Running = &running

	d.Port = pickPort(c, app, label(l, "port"))
	d.URLs = store.URLs{
		LAN:       firstNonEmpty(label(l, "url.lan"), label(l, "url")),
		Tailscale: label(l, "url.tailscale"),
		Public:    label(l, "url.public"),
	}
	if d.URLs == (store.URLs{}) {
		if d.Port == 0 {
			return Detected{}, false
		}
		d.URLs.LAN = schemeFor(d.Port) + "://{host}:" + strconv.Itoa(d.Port)
	}

	d.Name = firstNonEmpty(label(l, "name"), appField(app, func(a *App) string { return a.Name }), prettyName(composeService(c, l)))
	d.Icon = firstNonEmpty(label(l, "icon"), appField(app, func(a *App) string { return a.Icon }))
	d.Group = strings.ToLower(firstNonEmpty(label(l, "group"), appField(app, func(a *App) string { return a.Group })))
	d.Description = label(l, "description")
	d.Public = strings.EqualFold(label(l, "public"), "true")
	if app != nil {
		d.Integration = app.Integration
	}

	d.CheckType = firstNonEmpty(strings.ToLower(label(l, "check")), "http")
	path := label(l, "check.path")
	if path == "" && app != nil && app.Check != nil {
		path = app.Check.Path
	}
	d.CheckURL = checkURL(d, path)
	return d, true
}

// containerKey prefers an explicit id label, then the Compose project and
// service (stable across recreation), then the container name. Container
// IDs are never used: they change when a container is recreated.
func containerKey(c Container) string {
	if id := label(c.Labels, "id"); id != "" {
		return "label:" + id
	}
	project, service := c.Labels["com.docker.compose.project"], c.Labels["com.docker.compose.service"]
	if project != "" && service != "" {
		return "docker:" + project + "/" + service
	}
	return "docker:" + c.Name()
}

func composeService(c Container, l map[string]string) string {
	if s := l["com.docker.compose.service"]; s != "" {
		return s
	}
	return c.Name()
}

func isHaepage(c Container) bool {
	return NormalizeImage(c.Image) == "haelahpx/haepage" || strings.HasSuffix(NormalizeImage(c.Image), "/haepage")
}

// pickPort chooses the host port to link to: the label, then the known
// app's port, then the lowest published TCP port. Ports bound only to
// loopback are skipped, since other devices can't reach them.
func pickPort(c Container, app *App, labelPort string) int {
	type pub struct{ private, public int }
	var ports []pub
	seen := map[int]bool{}
	for _, p := range c.Ports {
		if p.PublicPort == 0 || p.Type != "tcp" || p.IP == "127.0.0.1" || p.IP == "::1" || seen[p.PublicPort] {
			continue
		}
		seen[p.PublicPort] = true
		ports = append(ports, pub{p.PrivatePort, p.PublicPort})
	}
	if v, err := strconv.Atoi(labelPort); err == nil && v > 0 {
		for _, p := range ports {
			if p.private == v || p.public == v {
				return p.public
			}
		}
		return v
	}
	if len(ports) == 0 {
		return 0
	}
	if app != nil {
		for _, p := range ports {
			if p.private == app.Port {
				return p.public
			}
		}
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i].public < ports[j].public })
	return ports[0].public
}

func schemeFor(port int) string {
	if port == 443 || port == 8443 {
		return "https"
	}
	return "http"
}

// checkURL is the address the server checks. "{host}" stays in it and is
// filled in at check time with the check host (127.0.0.1 normally, the
// Docker host when Haepage itself runs in a container).
func checkURL(d Detected, path string) string {
	u := firstNonEmpty(d.URLs.LAN, d.URLs.Public, d.URLs.Tailscale)
	if path != "" {
		u = strings.TrimRight(u, "/") + "/" + strings.TrimLeft(path, "/")
	}
	return u
}

// prettyName turns "spotify-lyrics-api" into "Spotify Lyrics Api".
func prettyName(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == '.' || r == ' ' })
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

func appField(app *App, f func(*App) string) string {
	if app == nil {
		return ""
	}
	return f(app)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

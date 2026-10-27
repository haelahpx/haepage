package discovery

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed known-apps.json
var knownAppsJSON []byte

// App is one entry of the known-apps list (schema/known-apps.schema.json).
type App struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Images      []string `json:"images"`
	Port        int      `json:"port"`
	Icon        string   `json:"icon"`
	Group       string   `json:"group"`
	Integration string   `json:"integration,omitempty"`
	Check       *struct {
		Path string `json:"path"`
	} `json:"check,omitempty"`
}

// KnownApps matches container images to apps.
type KnownApps struct {
	exact  map[string]*App // full normalized image → app
	last   map[string]*App // last path segment → app, for forks and mirrors
	byPort map[int]*App    // default port → app, only for ports a single app uses
}

// ByUniquePort returns the app that alone uses this default port, so a
// bare listening port can still get a proper name. Ports shared by
// several apps (80, 8080, 3000, ...) return nil.
func (k *KnownApps) ByUniquePort(port int) *App {
	if k == nil {
		return nil
	}
	return k.byPort[port]
}

// LoadKnownApps parses the embedded list.
func LoadKnownApps() (*KnownApps, error) {
	return ParseKnownApps(knownAppsJSON)
}

func ParseKnownApps(data []byte) (*KnownApps, error) {
	var file struct {
		Apps []App `json:"apps"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	k := &KnownApps{exact: map[string]*App{}, last: map[string]*App{}, byPort: map[int]*App{}}
	portUsers := map[int]int{}
	for i := range file.Apps {
		portUsers[file.Apps[i].Port]++
	}
	for i := range file.Apps {
		app := &file.Apps[i]
		if app.Port > 0 && portUsers[app.Port] == 1 {
			k.byPort[app.Port] = app
		}
		for _, img := range app.Images {
			n := NormalizeImage(img)
			k.exact[n] = app
			seg := n[strings.LastIndex(n, "/")+1:]
			if _, taken := k.last[seg]; !taken {
				k.last[seg] = app
			}
		}
	}
	return k, nil
}

// Match finds the app for an image reference, or nil.
func (k *KnownApps) Match(image string) *App {
	n := NormalizeImage(image)
	if app, ok := k.exact[n]; ok {
		return app
	}
	return k.last[n[strings.LastIndex(n, "/")+1:]]
}

// NormalizeImage strips the registry host, tag and digest, and Docker
// Hub's "library/" prefix: "lscr.io/linuxserver/radarr:latest" becomes
// "linuxserver/radarr".
func NormalizeImage(ref string) string {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		ref = ref[:i]
	}
	if first, rest, ok := strings.Cut(ref, "/"); ok && (strings.ContainsAny(first, ".:") || first == "localhost") {
		ref = rest
	}
	return strings.TrimPrefix(ref, "library/")
}

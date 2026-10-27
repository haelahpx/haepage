// Package rdap looks up when domains expire, using RDAP (the successor to
// WHOIS). IANA's bootstrap file says which registry answers for each
// top-level domain, and Haepage asks that registry directly.
package rdap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	bootstrapURL = "https://data.iana.org/rdap/dns.json"
	bootstrapFor = 24 * time.Hour
	answerFor    = 12 * time.Hour
)

var (
	// ErrNoRDAP means the domain's registry has no RDAP service.
	ErrNoRDAP = errors.New("this domain's registry doesn't offer RDAP")
	// ErrName means the input isn't a domain name.
	ErrName  = errors.New("not a domain name")
	hostname = regexp.MustCompile(`^(?i)([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{1,62}$`)
)

// Domain is the answer for one name.
type Domain struct {
	Name      string     `json:"name"`
	Expires   *time.Time `json:"expires,omitempty"`
	Registrar string     `json:"registrar,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// Client looks up domains. Its HTTP client should dial through the SSRF
// guard. Answers are cached, since expiry dates rarely change.
type Client struct {
	HTTP      *http.Client
	Bootstrap string // tests point this at a fake; IANA's file otherwise

	now func() time.Time

	mu       sync.Mutex
	servers  map[string]string // tld → base URL
	loaded   time.Time
	answers  map[string]Domain
	answered map[string]time.Time
}

func (c *Client) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// Lookup returns when name expires. A subdomain that the registry doesn't
// know is retried with its parent, down to two labels.
func (c *Client) Lookup(ctx context.Context, name string) Domain {
	name = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	if !hostname.MatchString(name) {
		return Domain{Name: name, Error: ErrName.Error()}
	}
	c.mu.Lock()
	if at, ok := c.answered[name]; ok && c.clock().Sub(at) < answerFor {
		d := c.answers[name]
		c.mu.Unlock()
		return d
	}
	c.mu.Unlock()

	d := Domain{Name: name}
	expires, registrar, err := c.lookup(ctx, name)
	if err != nil {
		d.Error = err.Error()
	} else {
		d.Expires, d.Registrar = expires, registrar
	}
	c.mu.Lock()
	if c.answers == nil {
		c.answers, c.answered = map[string]Domain{}, map[string]time.Time{}
	}
	c.answers[name], c.answered[name] = d, c.clock()
	c.mu.Unlock()
	return d
}

func (c *Client) lookup(ctx context.Context, name string) (*time.Time, string, error) {
	base, err := c.server(ctx, name)
	if err != nil {
		return nil, "", err
	}
	for candidate := name; strings.Count(candidate, ".") >= 1; {
		var doc struct {
			Events []struct {
				Action string `json:"eventAction"`
				Date   string `json:"eventDate"`
			} `json:"events"`
			Entities []struct {
				Roles  []string        `json:"roles"`
				VCard  json.RawMessage `json:"vcardArray"`
				Handle string          `json:"handle"`
			} `json:"entities"`
		}
		status, err := c.getJSON(ctx, strings.TrimSuffix(base, "/")+"/domain/"+candidate, &doc)
		if status == http.StatusNotFound && strings.Count(candidate, ".") > 1 {
			_, candidate, _ = strings.Cut(candidate, ".")
			continue
		}
		if status == http.StatusNotFound {
			return nil, "", errors.New("the registry doesn't know this domain")
		}
		if err != nil {
			return nil, "", err
		}
		var expires *time.Time
		for _, e := range doc.Events {
			if strings.EqualFold(e.Action, "expiration") {
				if t, err := time.Parse(time.RFC3339, e.Date); err == nil {
					t = t.UTC()
					expires = &t
				}
			}
		}
		var registrar string
		for _, e := range doc.Entities {
			for _, r := range e.Roles {
				if r == "registrar" {
					registrar = vcardName(e.VCard)
				}
			}
		}
		if expires == nil {
			return nil, registrar, errors.New("the registry doesn't publish an expiry date")
		}
		return expires, registrar, nil
	}
	return nil, "", ErrName
}

// server finds the RDAP base URL for name's top-level domain.
func (c *Client) server(ctx context.Context, name string) (string, error) {
	c.mu.Lock()
	stale := c.servers == nil || c.clock().Sub(c.loaded) > bootstrapFor
	c.mu.Unlock()
	if stale {
		var boot struct {
			Services [][][]string `json:"services"`
		}
		src := c.Bootstrap
		if src == "" {
			src = bootstrapURL
		}
		if _, err := c.getJSON(ctx, src, &boot); err != nil {
			return "", fmt.Errorf("reading the RDAP directory: %w", err)
		}
		servers := map[string]string{}
		for _, svc := range boot.Services {
			if len(svc) < 2 || len(svc[1]) == 0 {
				continue
			}
			url := svc[1][0]
			for _, u := range svc[1] { // prefer https
				if strings.HasPrefix(u, "https://") {
					url = u
					break
				}
			}
			for _, tld := range svc[0] {
				servers[strings.ToLower(tld)] = url
			}
		}
		c.mu.Lock()
		c.servers, c.loaded = servers, c.clock()
		c.mu.Unlock()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Longest match first, for entries like "co.uk" (rare in the file).
	labels := strings.Split(name, ".")
	for i := 1; i < len(labels); i++ {
		if url, ok := c.servers[strings.Join(labels[i:], ".")]; ok {
			return url, nil
		}
	}
	return "", ErrNoRDAP
}

func (c *Client) getJSON(ctx context.Context, url string, v any) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return 0, errors.New("the registry isn't answering")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, fmt.Errorf("the registry answered HTTP %d", res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(v); err != nil {
		return res.StatusCode, errors.New("the registry's answer can't be read")
	}
	return res.StatusCode, nil
}

// vcardName reads "fn" from a jCard: ["vcard", [["fn", {}, "text", "Name"], …]].
func vcardName(raw json.RawMessage) string {
	var card []json.RawMessage
	if json.Unmarshal(raw, &card) != nil || len(card) < 2 {
		return ""
	}
	var props [][]json.RawMessage
	if json.Unmarshal(card[1], &props) != nil {
		return ""
	}
	for _, p := range props {
		var key, value string
		if len(p) >= 4 && json.Unmarshal(p[0], &key) == nil && key == "fn" && json.Unmarshal(p[3], &value) == nil {
			return value
		}
	}
	return ""
}

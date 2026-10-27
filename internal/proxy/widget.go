package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/feeds"
	"github.com/haelahpx/haepage/internal/rdap"
)

const (
	maxBody        = 2 << 20 // bytes (IMPLEMENTATION.md §7.7)
	fetchTimeout   = 10 * time.Second
	defaultRefresh = 60
	minRefresh     = 5
)

var (
	// ErrNoWidget means there is no API widget with that id.
	ErrNoWidget = errors.New("no such API widget")
	// ErrWidget means the widget's settings or the API's answer can't be
	// used. Errors match it with errors.Is; their text is only the reason.
	ErrWidget = errors.New("widget error")

	// SecretName is the pattern for widget secret names. It has no ':', so
	// widgets can't reach integration keys ("int:…") or webhook tokens.
	SecretName = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	secretRef  = regexp.MustCompile(`\{\{\s*secrets\.([a-z0-9_-]{1,64})\s*\}\}`)
)

// APIOptions are the options of a widget of type "api" that the server
// uses. Fields and display are the browser's business.
type APIOptions struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// FeedOptions are the options of the feed ("rss"), "calendar" and
// "releases" widgets.
type FeedOptions struct {
	URL     string   `json:"url"`
	Limit   int      `json:"limit"`
	Days    int      `json:"days"`    // calendar: how far ahead
	Repos   []string `json:"repos"`   // releases: "owner/name"
	Domains []string `json:"domains"` // expiry: domain names to look up
}

// Widgets fetches data for the widgets the server answers for: "api",
// "rss", "calendar" and "releases". It fills in {{secrets.x}} from
// secrets.json, fetches through the SSRF guard, and caches the answer for
// the widget's refresh time. Secret values never leave the server: they are
// scrubbed from answers and error messages.
type Widgets struct {
	Config  func() config.Config
	Secrets interface{ Get(string) string }
	Client  *http.Client // dial through Guard
	RDAP    *rdap.Client // domain expiry lookups; nil turns "expiry" domains off

	now         func() time.Time
	releaseBase string // tests point this at a fake; "https://github.com" otherwise
	mu          sync.Mutex
	cache       map[string]cached
}

type cached struct {
	key     string // changes when the widget's settings change
	body    []byte
	expires time.Time
}

// Data returns the JSON answer for the widget id.
func (w *Widgets) Data(ctx context.Context, id string) ([]byte, error) {
	widget, err := w.find(id)
	if err != nil {
		return nil, err
	}
	refresh := widget.Refresh
	if refresh == 0 {
		refresh = defaultRefresh
		if widget.Type == "expiry" {
			refresh = 3600 // expiry dates change rarely; RDAP answers are cached longer still
		}
	}
	refresh = max(refresh, minRefresh)
	sum := sha256.Sum256(append([]byte(widget.Type), widget.Options...))
	key := hex.EncodeToString(sum[:])

	now := w.clock()
	w.mu.Lock()
	if c, ok := w.cache[id]; ok && c.key == key && now.Before(c.expires) {
		w.mu.Unlock()
		return c.body, nil
	}
	w.mu.Unlock()

	used := map[string]string{}
	fill := func(s string) (string, error) {
		var missing string
		out := secretRef.ReplaceAllStringFunc(s, func(ref string) string {
			name := secretRef.FindStringSubmatch(ref)[1]
			v := w.Secrets.Get(name)
			if v == "" && missing == "" {
				missing = name
			}
			used[name] = v
			return v
		})
		if missing != "" {
			return "", widgetErr("the secret %q is not set (add it in Settings → Secrets)", missing)
		}
		return out, nil
	}

	body, err := w.answer(ctx, widget, fill)
	if err != nil {
		return nil, redactErr(err, used)
	}
	body = redact(body, used)
	w.mu.Lock()
	if w.cache == nil {
		w.cache = map[string]cached{}
	}
	w.cache[id] = cached{key: key, body: body, expires: now.Add(time.Duration(refresh) * time.Second)}
	w.mu.Unlock()
	return body, nil
}

// serverTypes are the widget types whose data comes through the server.
var serverTypes = map[string]bool{"api": true, "rss": true, "calendar": true, "releases": true, "expiry": true}

func (w *Widgets) find(id string) (config.Widget, error) {
	for _, wd := range w.Config().Widgets {
		if wd.ID == id && serverTypes[wd.Type] {
			return wd, nil
		}
	}
	return config.Widget{}, ErrNoWidget
}

// answer fetches and shapes the data for one widget.
func (w *Widgets) answer(ctx context.Context, widget config.Widget, fill func(string) (string, error)) ([]byte, error) {
	if widget.Type == "api" {
		var opts APIOptions
		if err := unmarshalOptions(widget, &opts); err != nil {
			return nil, err
		}
		body, err := w.get(ctx, opts.URL, opts.Headers, "application/json", fill)
		if err != nil {
			return nil, err
		}
		if !json.Valid(body) {
			return nil, widgetErr("the answer isn't JSON")
		}
		return body, nil
	}

	var opts FeedOptions
	if err := unmarshalOptions(widget, &opts); err != nil {
		return nil, err
	}
	var out any
	switch widget.Type {
	case "rss":
		body, err := w.get(ctx, opts.URL, nil, feedAccept, fill)
		if err != nil {
			return nil, err
		}
		feed, err := feeds.ParseFeed(body, cmpInt(opts.Limit, 8))
		if err != nil {
			return nil, widgetErr("the address isn't an RSS or Atom feed")
		}
		out = feed
	case "calendar":
		link := opts.URL
		if rest, ok := strings.CutPrefix(link, "webcal://"); ok {
			link = "https://" + rest
		}
		body, err := w.get(ctx, link, nil, "text/calendar, */*;q=0.5", fill)
		if err != nil {
			return nil, err
		}
		now := w.clock()
		events, err := feeds.ParseICS(body, now, now.AddDate(0, 0, min(cmpInt(opts.Days, 14), 366)), cmpInt(opts.Limit, 10))
		if err != nil {
			return nil, widgetErr("the address isn't an iCalendar (.ics) file")
		}
		out = map[string]any{"events": events}
	case "releases":
		out = w.releases(ctx, opts)
	case "expiry":
		domains := []rdap.Domain{}
		for i, name := range opts.Domains {
			if i == 20 {
				break
			}
			if w.RDAP == nil {
				domains = append(domains, rdap.Domain{Name: name, Error: "domain lookups are off"})
				continue
			}
			domains = append(domains, w.RDAP.Lookup(ctx, name))
		}
		out = map[string]any{"domains": domains}
	}
	return json.Marshal(out)
}

const feedAccept = "application/rss+xml, application/atom+xml, application/xml;q=0.9, text/xml;q=0.9, */*;q=0.5"

var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// Release is one entry of the release tracker.
type Release struct {
	Repo string `json:"repo"`
	feeds.Item
}

// releases reads each repository's public releases feed (no token needed)
// and merges them, newest first. A repository that fails is reported in
// "errors" without hiding the others.
func (w *Widgets) releases(ctx context.Context, opts FeedOptions) map[string]any {
	items := []Release{}
	failed := []map[string]string{}
	repos := opts.Repos
	if len(repos) > 20 {
		repos = repos[:20]
	}
	for _, repo := range repos {
		repo = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(repo, "https://github.com/"), "/"))
		if !repoName.MatchString(repo) {
			failed = append(failed, map[string]string{"repo": repo, "error": "use owner/name"})
			continue
		}
		base := w.releaseBase
		if base == "" {
			base = "https://github.com"
		}
		body, err := w.get(ctx, base+"/"+repo+"/releases.atom", nil, feedAccept, func(s string) (string, error) { return s, nil })
		if err == nil {
			var feed feeds.Feed
			if feed, err = feeds.ParseFeed(body, 3); err == nil {
				for _, it := range feed.Items {
					items = append(items, Release{Repo: repo, Item: it})
				}
				continue
			}
		}
		failed = append(failed, map[string]string{"repo": repo, "error": "not found or not answering"})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Date, items[j].Date
		return a != nil && (b == nil || a.After(*b))
	})
	if limit := cmpInt(opts.Limit, 6); len(items) > limit {
		items = items[:limit]
	}
	return map[string]any{"items": items, "errors": failed}
}

func unmarshalOptions(widget config.Widget, v any) error {
	if len(widget.Options) > 0 && json.Unmarshal(widget.Options, v) != nil {
		return widgetErr("its options can't be read")
	}
	return nil
}

func cmpInt(n, fallback int) int {
	if n <= 0 {
		return fallback
	}
	return n
}

// get fetches link (after filling in secrets) through the guard and
// returns the body of a 2xx answer.
func (w *Widgets) get(ctx context.Context, link string, headers map[string]string, accept string, fill func(string) (string, error)) ([]byte, error) {
	rawURL, err := fill(link)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, widgetErr("the address must start with http:// or https://")
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, widgetErr("%v", err)
	}
	req.Header.Set("Accept", accept)
	for name, value := range headers {
		if strings.EqualFold(name, "Host") {
			continue
		}
		v, err := fill(value)
		if err != nil {
			return nil, err
		}
		req.Header.Set(name, v)
	}
	res, err := w.Client.Do(req)
	if err != nil {
		// Drop url.Error's `Get "…":` prefix; the reason is what helps.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("could not reach it: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("could not read the answer: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, widgetErr("it answered HTTP %d", res.StatusCode)
	}
	if len(body) > maxBody {
		return nil, widgetErr("the answer is larger than %d MB", maxBody>>20)
	}
	return body, nil
}

type widgetError struct{ msg string }

func (e widgetError) Error() string        { return e.msg }
func (e widgetError) Is(target error) bool { return target == ErrWidget }

func widgetErr(format string, args ...any) error {
	return widgetError{msg: fmt.Sprintf(format, args...)}
}

func (w *Widgets) clock() time.Time {
	if w.now != nil {
		return w.now()
	}
	return time.Now()
}

// redact replaces every secret value (and its URL-escaped form) with
// "[secret]". Applied to answers and errors so an API that echoes a header
// or a failing URL can't leak a secret to the browser.
func redact(b []byte, used map[string]string) []byte {
	for _, v := range used {
		if v == "" {
			continue
		}
		for _, form := range []string{v, url.QueryEscape(v), url.PathEscape(v)} {
			b = bytes.ReplaceAll(b, []byte(form), []byte("[secret]"))
		}
	}
	return b
}

type redactedError struct {
	msg string
	err error
}

func (e redactedError) Error() string { return e.msg }
func (e redactedError) Unwrap() error { return e.err }

func redactErr(err error, used map[string]string) error {
	return redactedError{msg: string(redact([]byte(err.Error()), used)), err: err}
}

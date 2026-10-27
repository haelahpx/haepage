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
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/haelahpx/haepage/internal/config"
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

// Widgets fetches data for API widgets: it fills in {{secrets.x}} from
// secrets.json, fetches through the SSRF guard, and caches the answer for
// the widget's refresh time. Secret values never leave the server: they are
// scrubbed from answers and error messages.
type Widgets struct {
	Config  func() config.Config
	Secrets interface{ Get(string) string }
	Client  *http.Client // dial through Guard

	now   func() time.Time
	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	key     string // changes when the widget's settings change
	body    []byte
	expires time.Time
}

// Data returns the JSON answer for the API widget id.
func (w *Widgets) Data(ctx context.Context, id string) ([]byte, error) {
	widget, opts, err := w.find(id)
	if err != nil {
		return nil, err
	}
	refresh := widget.Refresh
	if refresh == 0 {
		refresh = defaultRefresh
	}
	refresh = max(refresh, minRefresh)
	sum := sha256.Sum256(append([]byte(opts.URL), widget.Options...))
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

	body, err := w.fetch(ctx, opts, fill)
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

func (w *Widgets) find(id string) (config.Widget, APIOptions, error) {
	for _, wd := range w.Config().Widgets {
		if wd.ID != id || wd.Type != "api" {
			continue
		}
		var opts APIOptions
		if len(wd.Options) > 0 {
			if err := json.Unmarshal(wd.Options, &opts); err != nil {
				return wd, opts, widgetErr("its options can't be read")
			}
		}
		return wd, opts, nil
	}
	return config.Widget{}, APIOptions{}, ErrNoWidget
}

func (w *Widgets) fetch(ctx context.Context, opts APIOptions, fill func(string) (string, error)) ([]byte, error) {
	rawURL, err := fill(opts.URL)
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
	req.Header.Set("Accept", "application/json")
	for name, value := range opts.Headers {
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
		return nil, fmt.Errorf("could not reach the API: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("could not read the API's answer: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, widgetErr("the API answered HTTP %d", res.StatusCode)
	}
	if len(body) > maxBody {
		return nil, widgetErr("the answer is larger than %d MB", maxBody>>20)
	}
	ct, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if !json.Valid(body) {
		return nil, widgetErr("the answer isn't JSON (%s)", cmp(ct, "no content type"))
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

func cmp(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
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

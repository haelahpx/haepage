package themes

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
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/haelahpx/haepage/internal/config"
)

// MaxSize caps an imported theme. The theme budget is 50 KB (theme lint
// warns above it); this only refuses files that can't be a theme.
const MaxSize = 512 << 10

var (
	// ErrNotImported means the URL has no cached copy yet.
	ErrNotImported = errors.New("theme not imported")
	// ErrInvalid means the URL or the file it returned can't be a theme.
	ErrInvalid = errors.New("invalid theme")
)

// Info describes an imported theme. SHA256 is of the CSS, so two installs
// can tell they hold the same file.
type Info struct {
	URL       string    `json:"url"`
	SHA256    string    `json:"sha256"`
	Size      int       `json:"size"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// Remote fetches theme CSS from https:// links and keeps a copy in Dir,
// keyed by the sha256 of the URL. The copy is pinned: it is served as-is
// until the theme is imported again with refresh.
type Remote struct {
	Dir    string
	Client *http.Client // should dial through proxy.Guard

	mu sync.Mutex
}

var importRule = regexp.MustCompile(`(?i)@import\b`)

// CheckURL accepts only plain https:// links.
func CheckURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("%w: use an https:// link to a .css file", ErrInvalid)
	}
	return u, nil
}

// Validate checks the fetched bytes. Theme lint (M3.5) checks the budgets;
// this only rejects what must never be applied.
func Validate(css []byte) error {
	switch {
	case len(css) == 0:
		return fmt.Errorf("%w: the file is empty", ErrInvalid)
	case len(css) > MaxSize:
		return fmt.Errorf("%w: the file is larger than %d KB", ErrInvalid, MaxSize>>10)
	case !utf8.Valid(css) || bytes.IndexByte(css, 0) >= 0:
		return fmt.Errorf("%w: the file is not text", ErrInvalid)
	case bytes.HasPrefix(bytes.TrimSpace(bytes.ToLower(css[:min(len(css), 512)])), []byte("<")):
		return fmt.Errorf("%w: the link returned a web page, not CSS", ErrInvalid)
	case importRule.Match(css):
		// The browser would fetch @import targets itself, past the guard.
		return fmt.Errorf("%w: @import is not allowed in themes", ErrInvalid)
	}
	return nil
}

func (r *Remote) paths(rawURL string) (css, meta string) {
	sum := sha256.Sum256([]byte(rawURL))
	key := hex.EncodeToString(sum[:])
	return filepath.Join(r.Dir, key+".css"), filepath.Join(r.Dir, key+".json")
}

// Get returns the cached copy of an imported theme.
func (r *Remote) Get(rawURL string) ([]byte, Info, error) {
	cssPath, metaPath := r.paths(rawURL)
	var info Info
	meta, err := os.ReadFile(metaPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, info, ErrNotImported
	}
	if err != nil {
		return nil, info, err
	}
	if err := json.Unmarshal(meta, &info); err != nil || info.URL != rawURL {
		return nil, info, ErrNotImported
	}
	css, err := os.ReadFile(cssPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, info, ErrNotImported
	}
	if err != nil {
		return nil, info, err
	}
	sum := sha256.Sum256(css)
	if hex.EncodeToString(sum[:]) != info.SHA256 {
		return nil, info, ErrNotImported
	}
	return css, info, nil
}

// Import fetches and caches a theme. An already imported theme is returned
// from the cache unless refresh is set.
func (r *Remote) Import(ctx context.Context, rawURL string, refresh bool) (Info, error) {
	if _, err := CheckURL(rawURL); err != nil {
		return Info{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !refresh {
		if _, info, err := r.Get(rawURL); err == nil {
			return info, nil
		}
	}

	css, err := Fetch(ctx, r.Client, rawURL)
	if err != nil {
		return Info{}, err
	}
	if err := Validate(css); err != nil {
		return Info{}, err
	}
	sum := sha256.Sum256(css)
	info := Info{URL: rawURL, SHA256: hex.EncodeToString(sum[:]), Size: len(css), FetchedAt: time.Now().UTC()}
	meta, _ := json.Marshal(info)
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return Info{}, err
	}
	cssPath, metaPath := r.paths(rawURL)
	if err := config.WriteFileAtomic(cssPath, css, 0o644); err != nil {
		return Info{}, err
	}
	if err := config.WriteFileAtomic(metaPath, meta, 0o644); err != nil {
		return Info{}, err
	}
	return info, nil
}

// Fetch downloads theme CSS with a 10 s limit, refusing responses that
// aren't CSS or plain text. It reads at most MaxSize+1 bytes, so Validate
// can tell an oversized file apart.
func Fetch(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	req.Header.Set("Accept", "text/css,text/plain;q=0.8")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch the theme: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not fetch the theme: HTTP %d", res.StatusCode)
	}
	if ct, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type")); ct != "" && ct != "text/css" && ct != "text/plain" {
		return nil, fmt.Errorf("%w: the link returned %s, not CSS", ErrInvalid, ct)
	}
	css, err := io.ReadAll(io.LimitReader(res.Body, MaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("could not fetch the theme: %w", err)
	}
	return css, nil
}

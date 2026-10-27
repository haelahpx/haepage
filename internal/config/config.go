// Package config manages reading and writing haepage.json configuration
// and server-side secrets.json using atomic file operations.
package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Version is the config version this build reads and writes.
const Version = 1

// MaxSize is the largest config file accepted.
const MaxSize = 1 << 20

var (
	// ErrConflict means the file changed since the caller read it.
	ErrConflict = errors.New("config changed since it was read")
	// ErrInvalid wraps validation failures.
	ErrInvalid = errors.New("invalid config")
)

// URLs are a service's addresses. "{host}" in a URL stands for the host
// name the visitor used to reach Haepage.
type URLs struct {
	LAN       string `json:"lan,omitempty"`
	Tailscale string `json:"tailscale,omitempty"`
	Public    string `json:"public,omitempty"`
}

type Check struct {
	Type     string `json:"type,omitempty"`
	Expect   int    `json:"expect,omitempty"`
	Interval int    `json:"interval,omitempty"`
	Path     string `json:"path,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
}

type Service struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Group       string `json:"group,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
	URLs        URLs   `json:"urls"`
	Check       *Check `json:"check,omitempty"`
	Integration string `json:"integration,omitempty"` // e.g. "jellyfin", to connect a manual service
}

type Override struct {
	Name   string `json:"name,omitempty"`
	Icon   string `json:"icon,omitempty"`
	Group  string `json:"group,omitempty"`
	URLs   *URLs  `json:"urls,omitempty"`
	Pinned bool   `json:"pinned,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
}

type Settings struct {
	Inbox string `json:"inbox,omitempty"`
}

// Config is the part of haepage.json the server needs. The file itself is
// kept as raw JSON so fields the server doesn't know survive every save.
type Config struct {
	Version   int                 `json:"version"`
	Settings  Settings            `json:"settings"`
	Services  []Service           `json:"services"`
	Overrides map[string]Override `json:"overrides"`
}

// Store holds haepage.json in memory and writes it back atomically.
type Store struct {
	path string

	mu     sync.RWMutex
	raw    []byte
	parsed Config
	etag   string
	mtime  time.Time

	// OnChange is called after the config changes, from a save or from
	// an edit on disk.
	OnChange func(etag string)
}

const emptyConfig = `{"version":1}`

// Open loads the config at path. A missing file is not an error: the store
// starts empty and creates the file on the first save.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) reload() error {
	raw, err := os.ReadFile(s.path)
	var mtime time.Time
	switch {
	case errors.Is(err, os.ErrNotExist):
		raw = []byte(emptyConfig)
	case err != nil:
		return fmt.Errorf("read config: %w", err)
	default:
		if info, err := os.Stat(s.path); err == nil {
			mtime = info.ModTime()
		}
	}
	parsed, err := parse(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", s.path, err)
	}
	s.mu.Lock()
	s.raw, s.parsed, s.etag, s.mtime = raw, parsed, etagOf(raw), mtime
	s.mu.Unlock()
	return nil
}

func parse(raw []byte) (Config, error) {
	if len(raw) > MaxSize {
		return Config{}, fmt.Errorf("%w: larger than %d bytes", ErrInvalid, MaxSize)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return Config{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if c.Version == 0 {
		c.Version = Version
	}
	if c.Version > Version {
		return Config{}, fmt.Errorf("%w: version %d is newer than this Haepage (%d)", ErrInvalid, c.Version, Version)
	}
	if c.Overrides == nil {
		c.Overrides = map[string]Override{}
	}
	return c, nil
}

func etagOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}

// Get returns the raw file and its ETag.
func (s *Store) Get() ([]byte, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.raw, s.etag
}

// Config returns the parsed config.
func (s *Store) Config() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.parsed
}

// Put replaces the whole file. ifMatch must be the current ETag.
func (s *Store) Put(raw []byte, ifMatch string) (string, error) {
	parsed, err := parse(raw)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	if ifMatch != s.etag {
		s.mu.Unlock()
		return "", ErrConflict
	}
	etag, err := s.writeLocked(pretty(raw), parsed)
	s.mu.Unlock()
	if err == nil {
		s.changed(etag)
	}
	return etag, err
}

// SetOverride changes one entry under "overrides", keeping every other
// field of the file as it is. A nil override removes the entry.
func (s *Store) SetOverride(key string, o *Override) (string, error) {
	s.mu.Lock()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(s.raw, &top); err != nil {
		s.mu.Unlock()
		return "", fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	overrides := map[string]json.RawMessage{}
	if existing, ok := top["overrides"]; ok {
		_ = json.Unmarshal(existing, &overrides)
	}
	if o == nil {
		delete(overrides, key)
	} else {
		b, _ := json.Marshal(o)
		overrides[key] = b
	}
	top["overrides"], _ = json.Marshal(overrides)
	if _, ok := top["version"]; !ok {
		top["version"] = json.RawMessage("1")
	}
	raw, _ := json.MarshalIndent(top, "", "  ")
	raw = append(raw, '\n')
	parsed, err := parse(raw)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	etag, err := s.writeLocked(raw, parsed)
	s.mu.Unlock()
	if err == nil {
		s.changed(etag)
	}
	return etag, err
}

func (s *Store) changed(etag string) {
	if s.OnChange != nil {
		s.OnChange(etag)
	}
}

func pretty(raw []byte) []byte {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return raw
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}

// writeLocked writes the file atomically: a temp file in the same folder,
// fsync, rename over the old file, then fsync the folder. A crash at any
// point leaves either the old or the new file, never half of one.
func (s *Store) writeLocked(raw []byte, parsed Config) (string, error) {
	if err := WriteFileAtomic(s.path, raw, 0o644); err != nil {
		return "", err
	}
	s.raw, s.parsed, s.etag = raw, parsed, etagOf(raw)
	if info, err := os.Stat(s.path); err == nil {
		s.mtime = info.ModTime()
	}
	return s.etag, nil
}

// WriteFileAtomic replaces path with data without ever exposing a
// partially written file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".haepage-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

// Watch reloads the file when it changes on disk, checking its
// modification time every interval, until ctx ends. A file that fails to
// parse is ignored, keeping the last good config.
func (s *Store) Watch(ctx context.Context, interval time.Duration, onError func(error)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		info, err := os.Stat(s.path)
		if err != nil {
			continue
		}
		s.mu.RLock()
		same := info.ModTime().Equal(s.mtime)
		old := s.etag
		s.mu.RUnlock()
		if same {
			continue
		}
		if err := s.reload(); err != nil {
			if onError != nil {
				onError(err)
			}
			s.mu.Lock()
			s.mtime = info.ModTime() // don't report the same bad file every tick
			s.mu.Unlock()
			continue
		}
		if _, etag := s.Get(); etag != old {
			s.changed(etag)
		}
	}
}

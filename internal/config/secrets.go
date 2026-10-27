package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// Secrets is secrets.json: a flat {"name": "value"} map, readable only by
// the server (mode 0600). Values are never sent to the browser, logged or
// exported.
type Secrets struct {
	path string
	mu   sync.RWMutex
	m    map[string]string
}

// OpenSecrets loads secrets.json. A missing file means no secrets yet.
func OpenSecrets(path string) (*Secrets, error) {
	m, err := LoadSecrets(path)
	if err != nil {
		return nil, err
	}
	return &Secrets{path: path, m: m}, nil
}

// LoadSecrets reads a secrets file. A missing file means no secrets.
func LoadSecrets(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	secrets := map[string]string{}
	if err := json.Unmarshal(raw, &secrets); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return secrets, nil
}

func (s *Secrets) Get(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[name]
}

// Set stores a secret; an empty value deletes it. The file is rewritten
// atomically with mode 0600.
func (s *Secrets) Set(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[string]string, len(s.m)+1)
	for k, v := range s.m {
		next[k] = v
	}
	if value == "" {
		delete(next, name)
	} else {
		next[name] = value
	}
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(s.path, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	s.m = next
	return nil
}

// Names lists the secrets that are set.
func (s *Secrets) Names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.m))
	for k := range s.m {
		names = append(names, k)
	}
	return names
}

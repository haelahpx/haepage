package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenMissingFile(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "haepage.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, etag := s.Get()
	if string(raw) != emptyConfig || etag == "" {
		t.Errorf("got %q %q", raw, etag)
	}
	if s.Config().Version != 1 || s.Config().Overrides == nil {
		t.Errorf("config = %+v", s.Config())
	}
}

func TestOpenRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"bad.json": "{oops", "future.json": `{"version":9}`} {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte(body), 0o644)
		if _, err := Open(path); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestPutNeedsCurrentETag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "haepage.json")
	s, _ := Open(path)
	_, etag := s.Get()

	changes := 0
	s.OnChange = func(string) { changes++ }

	newTag, err := s.Put([]byte(`{"version":1,"settings":{"title":"Lab"}}`), etag)
	if err != nil {
		t.Fatal(err)
	}
	if newTag == etag || changes != 1 {
		t.Errorf("etag unchanged or OnChange not called (%d)", changes)
	}
	// The old ETag is now stale: a second tab saving with it must fail.
	if _, err := s.Put([]byte(`{"version":1}`), etag); !errors.Is(err, ErrConflict) {
		t.Errorf("stale put: err = %v, want ErrConflict", err)
	}
	disk, _ := os.ReadFile(path)
	if !strings.Contains(string(disk), `"title": "Lab"`) {
		t.Errorf("file = %s", disk)
	}
	if _, err := s.Put([]byte(`not json`), newTag); !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid put: err = %v", err)
	}
}

func TestSetOverrideKeepsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "haepage.json")
	os.WriteFile(path, []byte(`{"version":1,"futureField":{"a":1},"overrides":{"docker:x":{"name":"X"}}}`), 0o644)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetOverride("docker:media/app", &Override{Pinned: true, Group: "media"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := s.Get()
	var top map[string]json.RawMessage
	json.Unmarshal(raw, &top)
	if string(top["futureField"]) == "" {
		t.Error("unknown field lost")
	}
	o := s.Config().Overrides
	if !o["docker:media/app"].Pinned || o["docker:x"].Name != "X" {
		t.Errorf("overrides = %+v", o)
	}
	if _, err := s.SetOverride("docker:x", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Config().Overrides["docker:x"]; ok {
		t.Error("override not removed")
	}
}

func TestAtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "haepage.json")
	if err := WriteFileAtomic(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("folder has %d entries, want only the file", len(entries))
	}
	info, _ := os.Stat(path)
	if data, _ := os.ReadFile(path); string(data) != "two" || info.Mode().Perm() != 0o600 {
		t.Errorf("data = %q mode = %v", data, info.Mode().Perm())
	}
}

func TestWatchPicksUpEditsOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "haepage.json")
	os.WriteFile(path, []byte(`{"version":1}`), 0o644)
	s, _ := Open(path)
	got := make(chan string, 4)
	s.OnChange = func(etag string) { got <- etag }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 4)
	go s.Watch(ctx, 10*time.Millisecond, func(err error) { errs <- err })

	time.Sleep(20 * time.Millisecond)
	os.WriteFile(path, []byte(`{"version":1,"settings":{"inbox":"auto"}}`), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(path, future, future)

	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("edit on disk not noticed")
	}
	if s.Config().Settings.Inbox != "auto" {
		t.Errorf("config not reloaded: %+v", s.Config())
	}

	// A broken edit keeps the last good config and reports the error.
	os.WriteFile(path, []byte(`{broken`), 0o644)
	later := future.Add(time.Second)
	os.Chtimes(path, later, later)
	select {
	case <-errs:
	case <-time.After(2 * time.Second):
		t.Fatal("broken edit not reported")
	}
	if s.Config().Settings.Inbox != "auto" {
		t.Errorf("bad edit replaced the config: %+v", s.Config())
	}
}

func TestLoadSecrets(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadSecrets(filepath.Join(dir, "missing.json"))
	if err != nil || len(got) != 0 {
		t.Errorf("missing: %v %v", got, err)
	}
	path := filepath.Join(dir, "secrets.json")
	os.WriteFile(path, []byte(`{"api":"k"}`), 0o600)
	got, err = LoadSecrets(path)
	if err != nil || got["api"] != "k" {
		t.Errorf("got %v %v", got, err)
	}
}

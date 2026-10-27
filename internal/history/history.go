// Package history keeps 24 hours of health-check results per service, in
// 5-minute buckets, and saves them in cache/state.json so a restart keeps
// them (IMPLEMENTATION.md M4.6).
package history

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/haelahpx/haepage/internal/config"
)

const (
	Bucket  = 5 * time.Minute
	Buckets = 288 // 24 h
	// Bars is how many bars the browser gets (30 minutes each).
	Bars = 48
)

type counts struct {
	Checks uint16 `json:"c"`
	Fails  uint16 `json:"f"`
}

type series struct {
	Start   int64    `json:"start"` // Unix seconds of buckets[0]
	Buckets []counts `json:"buckets"`
}

// History records check results. It is safe for concurrent use.
type History struct {
	path string
	now  func() time.Time

	mu     sync.Mutex
	series map[string]*series
	dirty  bool
}

// Open loads path (cache/state.json). A missing or unreadable file starts
// an empty history: it's only a cache.
func Open(path string) *History {
	h := &History{path: path, now: time.Now, series: map[string]*series{}}
	var state struct {
		History map[string]*series `json:"history"`
	}
	if raw, err := os.ReadFile(path); err == nil && json.Unmarshal(raw, &state) == nil && state.History != nil {
		for id, s := range state.History {
			if s != nil && len(s.Buckets) == Buckets {
				h.series[id] = s
			}
		}
	}
	return h
}

func slot(t time.Time) int64 { return t.Unix() / int64(Bucket.Seconds()) }

// advance moves s forward so its last bucket is now's, dropping old ones.
func (s *series) advance(now time.Time) {
	end := slot(now)
	last := s.Start/int64(Bucket.Seconds()) + Buckets - 1
	if shift := end - last; shift > 0 {
		if shift >= Buckets {
			s.Buckets = make([]counts, Buckets)
		} else {
			s.Buckets = append(s.Buckets[shift:], make([]counts, shift)...)
		}
		s.Start = (end - Buckets + 1) * int64(Bucket.Seconds())
	}
}

// Record adds one check result ("up" or "down"; others are ignored).
func (h *History) Record(id, status string) {
	if status != "up" && status != "down" {
		return
	}
	now := h.now()
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.series[id]
	if s == nil {
		s = &series{Start: (slot(now) - Buckets + 1) * int64(Bucket.Seconds()), Buckets: make([]counts, Buckets)}
		h.series[id] = s
	}
	s.advance(now)
	b := &s.Buckets[Buckets-1]
	if b.Checks < 65535 {
		b.Checks++
		if status == "down" {
			b.Fails++
		}
	}
	h.dirty = true
}

// Summary is one service's last 24 hours. Bars has one character per
// 30 minutes, oldest first: "u" all up, "d" all down, "p" partly down,
// "." no checks. Uptime is the share of checks that were up (null when
// there were none).
type Summary struct {
	Uptime *float64 `json:"uptime"`
	Bars   string   `json:"bars"`
}

// Summaries returns a summary for every service with checks in the last
// 24 hours.
func (h *History) Summaries() map[string]Summary {
	now := h.now()
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[string]Summary{}
	per := Buckets / Bars
	for id, s := range h.series {
		s.advance(now)
		var checks, fails int
		var bars strings.Builder
		for i := 0; i < Bars; i++ {
			var c, f int
			for _, b := range s.Buckets[i*per : (i+1)*per] {
				c += int(b.Checks)
				f += int(b.Fails)
			}
			checks += c
			fails += f
			switch {
			case c == 0:
				bars.WriteByte('.')
			case f == 0:
				bars.WriteByte('u')
			case f == c:
				bars.WriteByte('d')
			default:
				bars.WriteByte('p')
			}
		}
		if checks == 0 {
			continue
		}
		up := float64(checks-fails) / float64(checks) * 100
		out[id] = Summary{Uptime: &up, Bars: bars.String()}
	}
	return out
}

// Save writes the history if it changed since the last save.
func (h *History) Save() error {
	h.mu.Lock()
	if !h.dirty {
		h.mu.Unlock()
		return nil
	}
	now := h.now()
	keep := map[string]*series{}
	for id, s := range h.series {
		s.advance(now)
		for _, b := range s.Buckets {
			if b.Checks > 0 {
				keep[id] = s
				break
			}
		}
	}
	h.series = keep
	raw, err := json.Marshal(map[string]any{"version": 1, "history": keep})
	h.dirty = false
	h.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir(h.path), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return config.WriteFileAtomic(h.path, raw, 0o644)
}

func dir(path string) string {
	if i := strings.LastIndexByte(path, os.PathSeparator); i > 0 {
		return path[:i]
	}
	return "."
}

package main

import (
	"strings"
	"sync"
	"time"

	"github.com/haelahpx/haepage/internal/config"
	"github.com/haelahpx/haepage/internal/discovery"
	"github.com/haelahpx/haepage/internal/health"
	"github.com/haelahpx/haepage/internal/store"
)

// services ties config, discovery and health together: whenever the
// config or the detected list changes, the merged list is rebuilt and the
// registry publishes the differences.
type services struct {
	cfg       *config.Store
	reg       *store.Registry
	isLocal   func(string) bool
	checkHost string // what "{host}" means for health checks

	mu       sync.Mutex
	detected []discovery.Detected
}

func (s *services) remerge() {
	s.mu.Lock()
	detected := s.detected
	s.mu.Unlock()
	s.reg.Set(discovery.Merge(s.cfg.Config(), detected, s.isLocal))
}

func (s *services) setDetected(d []discovery.Detected) {
	s.mu.Lock()
	s.detected = d
	s.mu.Unlock()
	s.remerge()
}

func (s *services) configChanged(etag string) {
	s.remerge()
	s.reg.Publish(store.Event{Type: "config", Data: map[string]string{"etag": etag}})
}

// targets lists what the health scheduler should check.
func (s *services) targets() []health.Target {
	list := s.reg.Snapshot()
	out := make([]health.Target, 0, len(list))
	for _, svc := range list {
		if svc.State == "hidden" || svc.CheckURL == "" {
			continue
		}
		out = append(out, health.Target{
			ID:       svc.ID,
			Type:     svc.CheckType,
			URL:      strings.ReplaceAll(svc.CheckURL, "{host}", s.checkHost),
			Expect:   svc.CheckExpect,
			Interval: time.Duration(svc.CheckInterval) * time.Second,
			Insecure: svc.CheckInsecure,
			Running:  svc.Running,
		})
	}
	return out
}

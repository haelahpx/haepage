package main

import (
	"testing"

	"github.com/haelahpx/haepage/internal/store"
)

func TestTargetsFillInTheCheckHost(t *testing.T) {
	reg := store.NewRegistry()
	reg.Set([]store.Service{
		{ID: "a", CheckType: "http", CheckURL: "http://{host}:8096/health", CheckInterval: 30},
		{ID: "b", State: "hidden", CheckURL: "http://{host}:1"},
		{ID: "c"}, // no URL to check
	})
	s := &services{reg: reg, checkHost: "host.docker.internal"}
	got := s.targets()
	if len(got) != 1 || got[0].URL != "http://host.docker.internal:8096/health" || got[0].Interval.Seconds() != 30 {
		t.Errorf("targets = %+v", got)
	}
}

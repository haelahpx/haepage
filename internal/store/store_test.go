package store

import (
	"sync"
	"testing"
)

func drain(ch <-chan Event) []Event {
	var out []Event
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, e)
		default:
			return out
		}
	}
}

func types(events []Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

func TestSetPublishesDifferences(t *testing.T) {
	r := NewRegistry()
	ch, stop := r.Subscribe()
	defer stop()

	r.Set([]Service{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}})
	if got := types(drain(ch)); len(got) != 2 || got[0] != "service.upsert" {
		t.Fatalf("events = %v", got)
	}

	// Same list again: nothing to publish.
	r.Set([]Service{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}})
	if got := drain(ch); len(got) != 0 {
		t.Errorf("unchanged list published %v", types(got))
	}

	// b renamed, a removed.
	r.Set([]Service{{ID: "b", Name: "Bee"}})
	got := types(drain(ch))
	if len(got) != 2 || got[0] != "service.remove" || got[1] != "service.upsert" {
		t.Errorf("events = %v", got)
	}
	if snap := r.Snapshot(); len(snap) != 1 || snap[0].Name != "Bee" {
		t.Errorf("snapshot = %+v", snap)
	}
}

func TestHealthIsKeptAndDeduplicated(t *testing.T) {
	r := NewRegistry()
	r.Set([]Service{{ID: "a"}})
	ch, stop := r.Subscribe()
	defer stop()

	r.SetHealth("a", Health{Status: "up", MS: 20})
	r.SetHealth("a", Health{Status: "up", MS: 40})  // same bucket: quiet
	r.SetHealth("a", Health{Status: "up", MS: 500}) // new bucket
	r.SetHealth("a", Health{Status: "down"})
	r.SetHealth("missing", Health{Status: "up"})
	if got := drain(ch); len(got) != 3 {
		t.Errorf("health events = %d, want 3", len(got))
	}

	// A new list without health keeps the last known result.
	r.Set([]Service{{ID: "a", Name: "renamed"}})
	if s, _ := r.Get("a"); s.Health == nil || s.Health.Status != "down" {
		t.Errorf("health lost: %+v", s.Health)
	}
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	r := NewRegistry()
	slow, _ := r.Subscribe()
	fast, stopFast := r.Subscribe()
	defer stopFast()

	// The fast subscriber reads each event as it arrives; the slow one never
	// reads, so its buffer fills and it is dropped.
	for i := 0; i < subscriberBuffer+10; i++ {
		r.Publish(Event{Type: "tick"})
		if _, ok := <-fast; !ok {
			t.Fatalf("fast subscriber dropped after %d events", i)
		}
	}

	n := 0
	for range slow {
		n++
	}
	if n != subscriberBuffer {
		t.Errorf("slow subscriber got %d events before being dropped, want %d", n, subscriberBuffer)
	}
	if r.Subscribers() != 1 {
		t.Errorf("subscribers = %d, want 1", r.Subscribers())
	}
}

func TestConcurrentUse(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, stop := r.Subscribe()
			drain(ch)
			stop()
			stop() // safe to call twice
		}()
	}
	for i := 0; i < 50; i++ {
		r.Set([]Service{{ID: "a", Name: string(rune('a' + i%26))}})
		r.SetHealth("a", Health{Status: "up", MS: int64(i * 50)})
	}
	wg.Wait()
}

func TestStoppedHealthClearsWhenRunningAgain(t *testing.T) {
	r := NewRegistry()
	stopped, running := false, true
	r.Set([]Service{{ID: "a", Running: &stopped, Health: &Health{Status: "down", Reason: StoppedReason}}})
	r.Set([]Service{{ID: "a", Running: &running}})
	if s, _ := r.Get("a"); s.Health != nil {
		t.Errorf("stale stopped health kept: %+v", s.Health)
	}
	r.SetHealth("a", Health{Status: "down", Reason: "HTTP 503"})
	r.Set([]Service{{ID: "a", Running: &running}})
	if s, _ := r.Get("a"); s.Health == nil || s.Health.Reason != "HTTP 503" {
		t.Errorf("real check result dropped: %+v", s.Health)
	}
}

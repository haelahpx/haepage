package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/store"
)

func server(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckHTTP(t *testing.T) {
	ctx := context.Background()
	ok := server(t, 200)
	auth := server(t, 401)
	broken := server(t, 503)

	cases := []struct {
		name   string
		target Target
		want   string
	}{
		{"200", Target{URL: ok.URL}, "up"},
		{"401 is running", Target{URL: auth.URL}, "up"},
		{"redirect is up", Target{URL: ok.URL + "/redirect"}, "up"},
		{"503 is down", Target{URL: broken.URL}, "down"},
		{"expect mismatch", Target{URL: ok.URL, Expect: 204}, "down"},
		{"expect match", Target{URL: auth.URL, Expect: 401}, "up"},
		{"refused", Target{URL: "http://127.0.0.1:1"}, "down"},
		{"bad URL", Target{URL: "::nope"}, "down"},
	}
	for _, c := range cases {
		if got := Check(ctx, c.target); got.Status != c.want {
			t.Errorf("%s: %+v, want %s", c.name, got, c.want)
		}
	}
}

func TestCheckStoppedContainer(t *testing.T) {
	stopped := false
	got := Check(context.Background(), Target{URL: "http://127.0.0.1:1", Running: &stopped})
	if got.Status != "down" || !strings.Contains(got.Reason, "not running") {
		t.Errorf("got %+v", got)
	}
}

func TestCheckTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	if got := Check(context.Background(), Target{Type: "tcp", URL: "tcp://" + ln.Addr().String()}); got.Status != "up" {
		t.Errorf("open port: %+v", got)
	}
	if got := Check(context.Background(), Target{Type: "tcp", URL: "127.0.0.1:1"}); got.Status != "down" {
		t.Errorf("closed port: %+v", got)
	}
}

func TestSchedulerChecksEveryTarget(t *testing.T) {
	ok := server(t, 200)
	var mu sync.Mutex
	results := map[string]string{}
	s := &Scheduler{
		Targets: func() []Target {
			return []Target{
				{ID: "a", URL: ok.URL},
				{ID: "b", URL: "http://127.0.0.1:1"},
				{ID: "c", Type: "none"},
			}
		},
		Report: func(id string, h store.Health) {
			mu.Lock()
			results[id] = h.Status
			mu.Unlock()
		},
		Workers: 2,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	deadline := time.After(6 * time.Second)
	for {
		mu.Lock()
		n := len(results)
		mu.Unlock()
		if n == 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("results after 6s: %v", results)
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	<-done
	if results["a"] != "up" || results["b"] != "down" {
		t.Errorf("results = %v", results)
	}
	if _, checked := results["c"]; checked {
		t.Error("type none was checked")
	}
}

func TestDoneSchedulesNextCheck(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &Scheduler{now: func() time.Time { return now }, next: map[string]time.Time{}, busy: map[string]bool{"a": true}}

	s.done(Target{ID: "a", Interval: 60 * time.Second}, store.Health{Status: "up"})
	if d := s.next["a"].Sub(now); d < 54*time.Second || d > 66*time.Second || s.busy["a"] {
		t.Errorf("up: next in %v, busy = %v; want 60s ±10%%", d, s.busy["a"])
	}
	s.done(Target{ID: "a", Interval: 60 * time.Second}, store.Health{Status: "down"})
	if d := s.next["a"].Sub(now); d > 17*time.Second {
		t.Errorf("down: next in %v, want about 15s", d)
	}
}

package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordAndSummaries(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	h := Open(filepath.Join(t.TempDir(), "cache", "state.json"))
	h.now = func() time.Time { return now }

	// One check a minute for the last 24 hours, with a 30-minute outage
	// that ended two hours ago.
	start := now.Add(-24 * time.Hour).Add(Bucket)
	outageFrom, outageTo := now.Add(-150*time.Minute), now.Add(-120*time.Minute)
	for at := start; !at.After(now); at = at.Add(time.Minute) {
		at := at
		h.now = func() time.Time { return at }
		status := "up"
		if !at.Before(outageFrom) && at.Before(outageTo) {
			status = "down"
		}
		h.Record("svc", status)
	}
	h.Record("svc", "unknown") // ignored
	h.now = func() time.Time { return now }

	s := h.Summaries()["svc"]
	if len(s.Bars) != Bars || s.Uptime == nil {
		t.Fatalf("summary = %+v", s)
	}
	// Bars count back from now, so a 30-minute outage usually straddles
	// two bars: one or two non-"u" bars about two hours before the end.
	bad := strings.Trim(s.Bars, "u")
	if (bad != "d" && bad != "pp" && bad != "dp" && bad != "pd") || !strings.HasSuffix(s.Bars, "uuuu") {
		t.Errorf("bars = %s", s.Bars)
	}
	if *s.Uptime < 97 || *s.Uptime > 98.5 {
		t.Errorf("uptime = %.2f, want about 97.9", *s.Uptime)
	}
	if _, ok := h.Summaries()["other"]; ok {
		t.Error("a service without checks must not be listed")
	}
}

func TestPartialAndAging(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	h := Open(filepath.Join(t.TempDir(), "state.json"))
	h.now = func() time.Time { return now }
	h.Record("a", "up")
	h.Record("a", "down")
	if b := h.Summaries()["a"].Bars; b[Bars-1] != 'p' || strings.Count(b, ".") != Bars-1 {
		t.Errorf("bars = %s", b)
	}
	// 25 hours later the old results have aged out.
	now = now.Add(25 * time.Hour)
	if _, ok := h.Summaries()["a"]; ok {
		t.Error("results older than 24 h must age out")
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "state.json")
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	h := Open(path)
	h.now = func() time.Time { return now }
	h.Record("a", "up")
	h.Record("b", "down")
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}

	again := Open(path)
	again.now = func() time.Time { return now.Add(10 * time.Minute) }
	got := again.Summaries()
	if len(got) != 2 || *got["a"].Uptime != 100 || *got["b"].Uptime != 0 {
		t.Errorf("after reload: %+v", got)
	}
	// The earlier bucket is still there, now one bar back or in the same one.
	if !strings.Contains(got["a"].Bars, "u") {
		t.Errorf("bars after reload = %s", got["a"].Bars)
	}

	// A broken file is only a cache: start empty.
	broken := filepath.Join(t.TempDir(), "state.json")
	if err := writeFile(broken, "{not json"); err != nil {
		t.Fatal(err)
	}
	if len(Open(broken).Summaries()) != 0 {
		t.Error("a broken state file must start empty")
	}
}

func writeFile(path, s string) error { return os.WriteFile(path, []byte(s), 0o644) }

package metrics

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/haelahpx/haepage/internal/system"
)

func TestWrite(t *testing.T) {
	cert := time.Unix(1_800_000_000, 0)
	up := 99.5
	snap := Snapshot{
		Version: "1.2.3",
		Services: []Service{
			{ID: "nas", Name: `My "NAS"`, Status: "up", MS: 42, CertExpires: &cert, Uptime24h: &up},
			{ID: "docker:media/app", Name: "Media\\Server\nTwo", Status: "down"},
			{ID: "port:9000", Name: "Unknown", Status: "unknown"},
		},
		Host: &system.Stats{
			CPU:     system.CPU{Percent: 12.5, Load1: 0.4},
			Memory:  system.Usage{Total: 16e9, Used: 8e9},
			Disks:   []system.Disk{{Path: "/", Total: 200e9, Used: 150e9}},
			Network: system.Network{RxRate: 1000, TxRate: 250.5},
			Temps:   []system.Sensor{{Name: "CPU", C: 51.5}},
			Uptime:  3600,
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, snap); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`haepage_build_info{version="1.2.3"} 1`,
		`haepage_services 3`,
		`haepage_service_up{id="docker:media/app",name="Media\\Server\nTwo"} 0`,
		`haepage_service_up{id="nas",name="My \"NAS\""} 1`,
		`haepage_service_response_seconds{id="nas",name="My \"NAS\""} 0.042`,
		`haepage_service_uptime_ratio{id="nas",name="My \"NAS\""} 0.995`,
		`haepage_service_cert_expiry_timestamp_seconds{id="nas",name="My \"NAS\""} 1.8e+09`,
		`haepage_host_cpu_ratio 0.125`,
		`haepage_host_memory_bytes{kind="total"} 1.6e+10`,
		`haepage_host_disk_bytes{path="/",kind="used"} 1.5e+11`,
		`haepage_host_network_bytes_per_second{direction="transmit"} 250.5`,
		`haepage_host_temperature_celsius{sensor="CPU"} 51.5`,
		`haepage_host_uptime_seconds 3600`,
	} {
		if !strings.Contains(out, want+"\n") {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, "port:9000") {
		t.Error("services that were never checked must not report up/down")
	}
	// Each metric family has exactly one HELP and one TYPE line, before its samples.
	for _, name := range []string{"haepage_service_up", "haepage_host_memory_bytes", "haepage_host_disk_bytes"} {
		if strings.Count(out, "# TYPE "+name+" ") != 1 || strings.Index(out, "# TYPE "+name+" ") > strings.Index(out, "\n"+name) {
			t.Errorf("%s: TYPE line missing, repeated or after its samples", name)
		}
	}
	// Every line is a comment or "name{labels} value".
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if !strings.HasPrefix(line, "# ") && strings.Count(line, " ") < 1 {
			t.Errorf("bad line %q", line)
		}
	}
}

func TestWriteWithoutHost(t *testing.T) {
	var buf bytes.Buffer
	Write(&buf, Snapshot{Version: "x"})
	if strings.Contains(buf.String(), "haepage_host_") || !strings.Contains(buf.String(), "haepage_services 0\n") {
		t.Errorf("out = %s", buf.String())
	}
}

func TestFormatValue(t *testing.T) {
	for v, want := range map[float64]string{1: "1", 0.042: "0.042", 1e10: "1e+10"} {
		if got := formatValue(v); got != want {
			t.Errorf("formatValue(%v) = %s, want %s", v, got, want)
		}
	}
}

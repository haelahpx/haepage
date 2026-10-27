// Package metrics writes Haepage's state in the Prometheus text exposition
// format (version 0.0.4), by hand, so no client library is needed
// (IMPLEMENTATION.md M4.8).
package metrics

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/haelahpx/haepage/internal/system"
)

// ContentType is the Prometheus text format's media type.
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

// Service is one service's latest state.
type Service struct {
	ID          string
	Name        string
	Status      string // up, down, unknown
	MS          int64
	CertExpires *time.Time
	Uptime24h   *float64 // percent
}

// Snapshot is everything one scrape reports. Host is nil when server stats
// are off.
type Snapshot struct {
	Version  string
	Services []Service
	Host     *system.Stats
}

type writer struct {
	w    io.Writer
	err  error
	seen map[string]bool
}

func (m *writer) printf(format string, args ...any) {
	if m.err == nil {
		_, m.err = fmt.Fprintf(m.w, format, args...)
	}
}

// metric writes HELP and TYPE once per name, then one sample.
func (m *writer) metric(name, typ, help string, labels []string, value float64) {
	if !m.seen[name] {
		m.seen[name] = true
		m.printf("# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
	}
	m.printf("%s%s %s\n", name, labelSet(labels), formatValue(value))
}

// labelSet renders key/value pairs as {k="v",…}, escaping values.
func labelSet(kv []string) string {
	if len(kv) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i+1 < len(kv); i += 2 {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(kv[i])
		b.WriteString(`="`)
		b.WriteString(escape(kv[i+1]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

var escaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

func escape(s string) string { return escaper.Replace(s) }

func formatValue(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "+Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	case math.IsNaN(v):
		return "NaN"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// Write writes s in the Prometheus text format.
func Write(w io.Writer, s Snapshot) error {
	m := &writer{w: w, seen: map[string]bool{}}
	m.metric("haepage_build_info", "gauge", "Haepage's version; always 1.", []string{"version", s.Version}, 1)

	services := append([]Service(nil), s.Services...)
	sort.Slice(services, func(i, j int) bool { return services[i].ID < services[j].ID })
	m.metric("haepage_services", "gauge", "Number of services on the page.", nil, float64(len(services)))
	for _, sv := range services {
		if sv.Status != "up" && sv.Status != "down" {
			continue
		}
		up := 0.0
		if sv.Status == "up" {
			up = 1
		}
		m.metric("haepage_service_up", "gauge", "1 if the service's last check passed, 0 if it failed.", []string{"id", sv.ID, "name", sv.Name}, up)
	}
	for _, sv := range services {
		if sv.Status == "up" && sv.MS > 0 {
			m.metric("haepage_service_response_seconds", "gauge", "How long the service's last check took.", []string{"id", sv.ID, "name", sv.Name}, float64(sv.MS)/1000)
		}
	}
	for _, sv := range services {
		if sv.Uptime24h != nil {
			m.metric("haepage_service_uptime_ratio", "gauge", "Share of checks in the last 24 hours that passed (0 to 1).", []string{"id", sv.ID, "name", sv.Name}, *sv.Uptime24h/100)
		}
	}
	for _, sv := range services {
		if sv.CertExpires != nil {
			m.metric("haepage_service_cert_expiry_timestamp_seconds", "gauge", "When the service's TLS certificate ends (Unix time).", []string{"id", sv.ID, "name", sv.Name}, float64(sv.CertExpires.Unix()))
		}
	}

	if h := s.Host; h != nil {
		m.metric("haepage_host_cpu_ratio", "gauge", "CPU in use on the server (0 to 1).", nil, h.CPU.Percent/100)
		m.metric("haepage_host_load1", "gauge", "One-minute load average.", nil, h.CPU.Load1)
		m.metric("haepage_host_memory_bytes", "gauge", "Server memory.", []string{"kind", "used"}, float64(h.Memory.Used))
		m.metric("haepage_host_memory_bytes", "gauge", "", []string{"kind", "total"}, float64(h.Memory.Total))
		for _, d := range h.Disks {
			m.metric("haepage_host_disk_bytes", "gauge", "Disk space.", []string{"path", d.Path, "kind", "used"}, float64(d.Used))
			m.metric("haepage_host_disk_bytes", "gauge", "", []string{"path", d.Path, "kind", "total"}, float64(d.Total))
		}
		m.metric("haepage_host_network_bytes_per_second", "gauge", "Network throughput.", []string{"direction", "receive"}, h.Network.RxRate)
		m.metric("haepage_host_network_bytes_per_second", "gauge", "", []string{"direction", "transmit"}, h.Network.TxRate)
		for _, t := range h.Temps {
			m.metric("haepage_host_temperature_celsius", "gauge", "Hottest reading per sensor device.", []string{"sensor", t.Name}, t.C)
		}
		m.metric("haepage_host_uptime_seconds", "gauge", "Time since the server booted.", nil, float64(h.Uptime))
	}
	return m.err
}

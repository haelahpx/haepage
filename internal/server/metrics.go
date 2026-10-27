package server

import (
	"log/slog"
	"net/http"

	"github.com/haelahpx/haepage/internal/metrics"
)

// metricsHandler serves GET /metrics: service health, 24-hour uptime and
// host stats for a Prometheus scraper. Hidden services are left out.
func metricsHandler(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := metrics.Snapshot{Version: opts.Version}
		var uptime map[string]float64
		if opts.History != nil {
			uptime = map[string]float64{}
			for id, s := range opts.History.Summaries() {
				if s.Uptime != nil {
					uptime[id] = *s.Uptime
				}
			}
		}
		if opts.Registry != nil {
			for _, sv := range opts.Registry.Snapshot() {
				if sv.State == "hidden" {
					continue
				}
				m := metrics.Service{ID: sv.ID, Name: sv.Name, Status: "unknown"}
				if h := sv.Health; h != nil {
					m.Status, m.MS, m.CertExpires = h.Status, h.MS, h.CertExpires
				}
				if u, ok := uptime[sv.ID]; ok {
					m.Uptime24h = &u
				}
				snap.Services = append(snap.Services, m)
			}
		}
		if opts.System != nil {
			if st, err := opts.System.Read(); err != nil {
				slog.Warn("reading system stats for metrics", "err", err)
			} else {
				snap.Host = &st
			}
		}
		w.Header().Set("Content-Type", metrics.ContentType)
		w.Header().Set("Cache-Control", "no-store")
		if err := metrics.Write(w, snap); err != nil {
			slog.Debug("writing metrics", "err", err)
		}
	}
}

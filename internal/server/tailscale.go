package server

import (
	"log/slog"
	"net/http"

	"github.com/haelahpx/haepage/internal/tailscale"
)

func tailscaleDevices(c *tailscale.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		devices, err := c.Devices(r.Context())
		if err != nil {
			slog.Warn("reading tailscale status", "err", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Tailscale isn't answering"})
			return
		}
		writeJSON(w, http.StatusOK, devices)
	}
}

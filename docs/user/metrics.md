# Metrics

Haepage can publish what it knows about your services and server at `/metrics`, in the standard plain-text metrics format (also known as OpenMetrics text). Most monitoring tools can read it on a schedule ("scrape" it) to draw graphs and send alerts.

It is **off by default**. Turn it on with the `--metrics` flag or `HAEPAGE_METRICS=true`:

```yaml
# docker-compose.yml, under the haepage service
environment:
  HAEPAGE_METRICS: "true"
```

With the install script, add `HAEPAGE_METRICS=true` to `/etc/haepage/env` and restart.

Then open `http://<your-server>:8080/metrics` to check it works. Point your monitoring tool at the same address; once a minute is plenty, since health checks don't run more often than that.

> `/metrics` has no login. It shows service names and ids, but never URLs, keys or secrets. If Haepage is reachable from the internet, block `/metrics` in your reverse proxy or leave it off.

## What it reports

Services you have hidden are left out. Every service metric has an `id` and a `name` label.

| Metric | What it is |
| --- | --- |
| `haepage_build_info{version}` | Always 1; the label is Haepage's version |
| `haepage_services` | Number of services on the page |
| `haepage_service_up` | 1 if the last health check passed, 0 if it failed. Services not checked yet are missing |
| `haepage_service_response_seconds` | How long the last passing check took |
| `haepage_service_uptime_ratio` | Share of checks in the last 24 hours that passed, 0 to 1 |
| `haepage_service_cert_expiry_timestamp_seconds` | When an https service's certificate ends (Unix time) |

With server stats on (the default; `--no-system` turns them off):

| Metric | What it is |
| --- | --- |
| `haepage_host_cpu_ratio` | CPU in use, 0 to 1 |
| `haepage_host_load1` | One-minute load average |
| `haepage_host_memory_bytes{kind="used"\|"total"}` | Memory |
| `haepage_host_disk_bytes{path, kind="used"\|"total"}` | Disk space per disk |
| `haepage_host_network_bytes_per_second{direction="receive"\|"transmit"}` | Network throughput |
| `haepage_host_temperature_celsius{sensor}` | Hottest reading per sensor |
| `haepage_host_uptime_seconds` | Time since the server booted |

## Alert ideas

- A service is down: `haepage_service_up == 0` for 5 minutes.
- A certificate ends within 14 days: `haepage_service_cert_expiry_timestamp_seconds - time() < 14 * 86400`.
- A disk is almost full: `haepage_host_disk_bytes{kind="used"} / ignoring(kind) haepage_host_disk_bytes{kind="total"} > 0.9`.

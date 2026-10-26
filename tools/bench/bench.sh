#!/usr/bin/env bash
# Measures Haepage's idle footprint: starts the binary, lets it settle,
# then samples resident memory and CPU over a quiet window.
#
#   tools/bench/bench.sh [binary] [settle-seconds] [window-seconds]
#
# Prints key=value lines so CI can store them and compare runs.
set -euo pipefail

bin=${1:-./haepage}
settle=${2:-10}
window=${3:-60}
addr=127.0.0.1:18081

"$bin" --addr "$addr" --log-level warn &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT

for _ in $(seq 50); do
  curl -fs "http://$addr/api/ping" >/dev/null && break
  sleep 0.1
done
curl -fs "http://$addr/" >/dev/null # warm the page path once

sleep "$settle"

ticks() { awk '{print $14 + $15}' "/proc/$pid/stat"; }
hz=$(getconf CLK_TCK)
start=$(ticks)
sleep "$window"
end=$(ticks)

rss_kb=$(awk '/^VmRSS:/ {print $2}' "/proc/$pid/status")
size=$(stat -c %s "$bin")

awk -v rss="$rss_kb" -v t="$((end - start))" -v hz="$hz" -v w="$window" -v size="$size" 'BEGIN {
  printf "idle_rss_mb=%.1f\n", rss / 1024
  printf "idle_cpu_pct=%.2f\n", t / hz / w * 100
  printf "binary_mb=%.1f\n", size / 1048576
}'

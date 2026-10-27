// Formatting for the server stats tile.

const UNITS = ["B", "KB", "MB", "GB", "TB", "PB"];

export function formatBytes(bytes) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), UNITS.length - 1);
  const v = bytes / 1024 ** i;
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${UNITS[i]}`;
}

export function formatRate(bytesPerSecond) {
  return `${formatBytes(bytesPerSecond)}/s`;
}

export function formatUptime(seconds) {
  if (!seconds || seconds < 60) return "just started";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export function percent(used, total) {
  return total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0;
}

// Picks the disk to show: the largest one.
export function mainDisk(disks = []) {
  return disks.reduce((best, d) => (!best || d.total > best.total ? d : best), null);
}

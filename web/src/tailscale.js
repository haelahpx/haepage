// Display helpers for the Tailscale devices tile (GET /api/tailscale/devices).

const OS_NAMES = { linux: "Linux", windows: "Windows", macOS: "macOS", iOS: "iOS", android: "Android", freebsd: "FreeBSD", tvOS: "tvOS" };

export function osName(os) {
  return OS_NAMES[os] ?? os ?? "";
}

// "3 of 5 online" (this server counts as a device).
export function onlineSummary(devices) {
  const online = devices.filter((d) => d.online).length;
  return `${online} of ${devices.length} online`;
}

// "seen 5m ago", "seen 3h ago", "seen 12d ago".
export function seenAgo(iso, now = Date.now()) {
  if (!iso) return "offline";
  const s = Math.max(0, (now - Date.parse(iso)) / 1000);
  if (s < 60) return "seen just now";
  if (s < 3600) return `seen ${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `seen ${Math.floor(s / 3600)}h ago`;
  return `seen ${Math.floor(s / 86400)}d ago`;
}

// The first IPv4 tailnet address, which is what people type.
export function mainIp(ips = []) {
  return ips.find((ip) => !ip.includes(":")) ?? ips[0] ?? "";
}

// The detail line under a device name.
export function deviceDetail(d, now) {
  return [osName(d.os), mainIp(d.ips), d.online ? d.connection === "relay" && "relayed" : seenAgo(d.lastSeen, now)]
    .filter(Boolean)
    .join(" · ");
}

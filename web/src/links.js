// Smart links (IMPLEMENTATION.md §6.5): pick the LAN, Tailscale or public
// URL of a service based on how the visitor reached this page.

const ORDER = {
  tailscale: ["tailscale", "lan", "public"],
  lan: ["lan", "tailscale", "public"],
  public: ["public", "tailscale", "lan"],
};

// Classifies the page's hostname as "lan", "tailscale" or "public".
export function networkOf(hostname = "") {
  const host = hostname.replace(/^\[|\]$/g, "").toLowerCase();
  if (host.endsWith(".ts.net")) return "tailscale";

  const v4 = host.match(/^(\d{1,3})\.(\d{1,3})\.\d{1,3}\.\d{1,3}$/);
  if (v4) {
    const a = Number(v4[1]);
    const b = Number(v4[2]);
    if (a === 100 && b >= 64 && b <= 127) return "tailscale"; // 100.64.0.0/10
    if (a === 10 || a === 127) return "lan";
    if (a === 172 && b >= 16 && b <= 31) return "lan";
    if (a === 192 && b === 168) return "lan";
    if (a === 169 && b === 254) return "lan";
    return "public";
  }

  if (host.includes(":")) {
    if (host.startsWith("fd7a:115c:a1e0:")) return "tailscale"; // Tailscale's IPv6 range
    if (host === "::1" || /^f[cd]/.test(host) || host.startsWith("fe80:")) return "lan";
    return "public";
  }

  if (host === "" || host === "localhost" || !host.includes(".")) return "lan";
  if (/\.(local|lan|home\.arpa|internal)$/.test(host)) return "lan";
  return "public";
}

// Returns the URL to open for a service, or null when it has none.
// "{host}" in a URL (used for detected services) becomes the host name the
// visitor used, so the same link works from the LAN or over Tailscale.
export function resolveLink(urls, hostname) {
  const network = networkOf(hostname);
  const kinds = ORDER[network].filter((kind) => urls?.[kind]);
  if (kinds.length === 0) return null;
  const host = hostname.includes(":") && !hostname.startsWith("[") ? `[${hostname}]` : hostname;
  const fill = (u) => u.replaceAll("{host}", host || "localhost");
  return {
    url: fill(urls[kinds[0]]),
    kind: kinds[0],
    network,
    others: kinds.slice(1).map((kind) => ({ kind, url: fill(urls[kind]) })),
  };
}

export function hostOf(url) {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

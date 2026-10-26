// Detects whether a Haepage server is behind this page. Static mode has no
// backend, so a failed, slow or foreign /api/ping means static.
export async function detectMode(fetchFn = fetch, timeoutMs = 1500) {
  try {
    const res = await fetchFn("/api/ping", {
      signal: AbortSignal.timeout(timeoutMs),
      cache: "no-store",
    });
    if (!res.ok) return staticMode();
    const ping = await res.json();
    if (ping?.app !== "haepage") return staticMode();
    return {
      mode: "server",
      version: ping.version,
      capabilities: new Set(ping.capabilities ?? []),
    };
  } catch {
    return staticMode();
  }
}

function staticMode() {
  return { mode: "static", version: null, capabilities: new Set() };
}

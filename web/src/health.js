// Browser health probes for Static mode (IMPLEMENTATION.md §6.3). A no-cors
// request can't read the status code, so any response counts as "up" and a
// network error or timeout as "down".

// Returns why a URL can't be probed from this page, or null if it can.
export function unprobeable(url, pageProtocol) {
  let target;
  try {
    target = new URL(url);
  } catch {
    return "invalid URL";
  }
  if (target.protocol !== "http:" && target.protocol !== "https:") return "not a web URL";
  // Browsers block http:// requests from https:// pages (mixed content).
  if (pageProtocol === "https:" && target.protocol === "http:") return "http:// services can't be checked from an https:// page";
  return null;
}

export async function probe(url, { fetchFn = fetch, timeoutMs = 4000 } = {}) {
  try {
    await fetchFn(url, { mode: "no-cors", cache: "no-store", signal: AbortSignal.timeout(timeoutMs) });
    return "up";
  } catch {
    return "down";
  }
}

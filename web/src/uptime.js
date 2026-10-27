// Uptime history from GET /api/history: per service {uptime, bars}, where
// bars has one character per 30 minutes: u up, d down, p partly down,
// . not checked.

// "100%", "99.9%", "97.25%": more digits only where they say something.
export function formatUptime(pct) {
  if (pct == null) return "";
  if (pct >= 100) return "100%";
  const digits = pct >= 99.9 ? 2 : pct >= 99 ? 1 : 0;
  // Never round up to 100%: a single failure should still show.
  const factor = 10 ** digits;
  return `${(Math.floor(pct * factor) / factor).toFixed(digits)}%`;
}

const BAR_STATE = { u: "up", d: "down", p: "partial", ".": "none" };

export function barStates(bars = "") {
  return [...bars].map((c) => BAR_STATE[c] ?? "none");
}

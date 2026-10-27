// Counts health results by status for the status tile.
export function summarize(health) {
  const counts = { total: 0, up: 0, down: 0, unknown: 0 };
  for (const { status } of Object.values(health)) {
    counts.total++;
    counts[status === "up" || status === "down" ? status : "unknown"]++;
  }
  return counts;
}

export function headline({ total, up, down }) {
  if (total === 0) return "No checks yet";
  if (down > 0) return `${down} ${down === 1 ? "service" : "services"} down`;
  if (up === total) return "All services up";
  if (up > 0) return "Everything checked is up";
  return "Checking…";
}

export function greeting(hour) {
  if (hour < 5) return "Good night";
  if (hour < 12) return "Good morning";
  if (hour < 18) return "Good afternoon";
  return "Good evening";
}

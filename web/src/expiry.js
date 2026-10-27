// Certificate and domain expiry: health.certExpires (https services) and
// the expiry widget's domain lookups.

const DAY = 86400_000;

export function daysLeft(iso, now = Date.now()) {
  return Math.floor((Date.parse(iso) - now) / DAY);
}

// A card chip when a service's certificate ends within `warnDays`.
export function certNotice(iso, now = Date.now(), warnDays = 14) {
  if (!iso) return null;
  const d = daysLeft(iso, now);
  if (Date.parse(iso) <= now) return { text: "Cert expired", tone: "warn", title: `The certificate ended on ${iso.slice(0, 10)}` };
  if (d < warnDays) return { text: d < 1 ? "Cert ends today" : `Cert ${d} d`, tone: "warn", title: `The certificate ends on ${iso.slice(0, 10)}` };
  return null;
}

// "in 23 days", "tomorrow", "today", "2 days ago".
export function whenText(iso, now = Date.now()) {
  if (Date.parse(iso) <= now) {
    const ago = Math.max(0, Math.floor((now - Date.parse(iso)) / DAY));
    return ago === 0 ? "ended today" : `ended ${ago} ${ago === 1 ? "day" : "days"} ago`;
  }
  const d = daysLeft(iso, now);
  if (d < 1) return "today";
  if (d === 1) return "tomorrow";
  return `in ${d} days`;
}

// Rows for the expiry widget: certificates from the page's services and
// looked-up domains, soonest first; entries without a date go last.
export function expiryRows(services = [], domains = []) {
  const rows = [];
  for (const s of services) {
    if (s.health?.certExpires) rows.push({ kind: "cert", name: s.name, expires: s.health.certExpires });
  }
  for (const d of domains) rows.push({ kind: "domain", name: d.name, expires: d.expires ?? null, error: d.error, registrar: d.registrar });
  return rows.sort((a, b) => (a.expires ? Date.parse(a.expires) : Infinity) - (b.expires ? Date.parse(b.expires) : Infinity));
}

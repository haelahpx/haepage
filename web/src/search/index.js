import { hostOf, resolveLink } from "../links.js";

// Builds the local search list from services and bookmarks.
export function buildIndex({ services = [], bookmarks = [], groups = [], hostname = "" }) {
  const groupName = new Map(groups.map((g) => [g.id, g.name]));
  const entries = [];

  for (const s of services) {
    const link = s.link ?? resolveLink(s.urls, hostname);
    if (!link) continue;
    entries.push(entry("service", s.id, s.name, link.url, groupName.get(s.group), [s.description, ...(s.tags ?? [])]));
  }
  for (const [i, b] of bookmarks.entries()) {
    if (!b?.name || !b.url) continue;
    entries.push(entry("bookmark", `bookmark-${i}`, b.name, b.url, groupName.get(b.group) ?? b.group, []));
  }
  return entries;
}

function entry(kind, id, name, url, group, extra) {
  const other = [group, hostOf(url), ...extra].filter(Boolean).map((t) => t.toLowerCase());
  return { kind, id, name, url, group, key: name.toLowerCase(), other };
}

// Scores one query word: prefix > word prefix > substring > other fields >
// in-order letters. Zero means no match.
function scoreToken(e, token) {
  if (e.key.startsWith(token)) return 100;
  if (e.key.split(/[\s\-_.]+/).some((w) => w.startsWith(token))) return 80;
  if (e.key.includes(token)) return 60;
  if (e.other.some((field) => field.includes(token))) return 40;
  return isSubsequence(token, e.key) ? 10 : 0;
}

function isSubsequence(needle, haystack) {
  let i = 0;
  for (const ch of haystack) {
    if (ch === needle[i]) i++;
    if (i === needle.length) return true;
  }
  return false;
}

// Returns the best matches for the query. Every word must match; ties keep
// config order.
export function search(entries, query, limit = 8) {
  const tokens = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return [];
  const scored = [];
  for (const [order, e] of entries.entries()) {
    let total = 0;
    for (const token of tokens) {
      const s = scoreToken(e, token);
      if (s === 0) {
        total = 0;
        break;
      }
      total += s;
    }
    if (total > 0) scored.push({ e, total, order });
  }
  scored.sort((a, b) => b.total - a.total || a.order - b.order);
  return scored.slice(0, limit).map((s) => s.e);
}

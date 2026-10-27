// Live data from connected apps: turning integration states into card
// badges, tiles and notifications.

export const KIND_NAMES = { jellyfin: "Jellyfin", seerr: "Jellyseerr / Overseerr", radarr: "Radarr", sonarr: "Sonarr" };

// The small badge on a service's card, or null.
export function chipFor(state) {
  if (!state) return null;
  if (state.status === "error") return { text: "!", tone: "warn", title: state.error };
  const d = state.data;
  if (!d) return null;
  switch (state.kind) {
    case "jellyfin":
      return d.sessions?.length ? { text: `▶ ${d.sessions.length}`, tone: "live", title: `${d.sessions.length} watching` } : null;
    case "seerr":
      return d.pending ? { text: `${d.pending} pending`, tone: "info", title: `${d.pending} requests waiting for approval` } : null;
    case "radarr":
    case "sonarr":
      if (d.health?.length) return { text: "!", tone: "warn", title: d.health.map((h) => h.message).join("\n") };
      return d.queue?.length ? { text: `↓ ${d.queue.length}`, tone: "info", title: `${d.queue.length} downloading` } : null;
    default:
      return null;
  }
}

const byKind = (states, ...kinds) => Object.values(states).filter((s) => kinds.includes(s.kind) && s.status === "ok" && s.data);

export function nowPlaying(states) {
  return byKind(states, "jellyfin").flatMap((s) => s.data.sessions.map((x) => ({ ...x, service: s.service })));
}

export function pendingRequests(states) {
  return byKind(states, "seerr")
    .flatMap((s) => s.data.requests.map((r) => ({ ...r, service: s.service })))
    .sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)));
}

export function downloads(states) {
  return byKind(states, "radarr", "sonarr").flatMap((s) => s.data.queue.map((q) => ({ ...q, service: s.service, kind: s.kind })));
}

export function upcoming(states) {
  return byKind(states, "radarr", "sonarr")
    .flatMap((s) => s.data.upcoming.map((u) => ({ ...u, service: s.service, kind: s.kind })))
    .sort((a, b) => String(a.date).localeCompare(String(b.date)))
    .slice(0, 8);
}

const DAY = 86_400_000;

// "Today", "Tomorrow", or a weekday for the coming week.
export function formatDay(date, now = new Date()) {
  const d = new Date(date);
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const days = Math.floor((d.getTime() - start) / DAY);
  if (days <= 0) return "Today";
  if (days === 1) return "Tomorrow";
  return d.toLocaleDateString([], { weekday: "long" });
}

export function timeAgo(date, now = new Date()) {
  const s = Math.max(0, Math.round((now - new Date(date)) / 1000));
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < DAY / 1000) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

export function formatClock(ms) {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const mm = String(m).padStart(h ? 2 : 1, "0");
  return h ? `${h}:${mm}:${String(s).padStart(2, "0")}` : `${mm}:${String(s).padStart(2, "0")}`;
}

// Playback starts can be frequent, so they only go to the activity feed.
const QUIET = new Set(["playback.start"]);

export function shouldNotify(activity) {
  return !QUIET.has(activity.type);
}

export function posterFor(session) {
  return `./api/integrations/image?service=${encodeURIComponent(session.service)}&item=${encodeURIComponent(session.itemId)}`;
}

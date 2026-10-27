// Built-in widget types. Each module is loaded with import() only when a
// widget of that type is on the page, so unused widgets cost nothing.
// `defaults` are the options a new widget starts with.
export const WIDGET_TYPES = {
  clock: { name: "Clock", defaults: { timezone: "", label: "" }, load: () => import("./clock.jsx") },
  weather: { name: "Weather", defaults: { units: "metric" }, load: () => import("./weather.jsx") },
  system: { name: "Server stats", defaults: {}, load: () => import("./system.jsx"), needs: "system" },
  bookmarks: { name: "Bookmarks", defaults: { links: [] }, load: () => import("./bookmarks.jsx") },
  notes: { name: "Notes", defaults: { text: "" }, load: () => import("./notes.jsx") },
  todo: { name: "To-do", defaults: { items: [] }, load: () => import("./todo.jsx") },
  api: { name: "API (no code)", defaults: { url: "", headers: {}, display: "stat", fields: [] }, load: () => import("./api.jsx"), needs: "widgets.proxy" },
};

// Builds a new widget entry for haepage.json with an id not in `taken`.
export function newWidget(type, taken = [], random = Math.random) {
  const used = new Set(taken);
  let id;
  do {
    id = `${type}-${random().toString(36).slice(2, 6)}`;
  } while (used.has(id));
  return { id, type, options: structuredClone(WIDGET_TYPES[type].defaults) };
}

// Where widgets go (IMPLEMENTATION.md §5.7 `area`): "header" (the default)
// sits with the tiles before the groups, a group id puts it inside that
// group, and "sidebar" (or a group that no longer exists) after the groups.
export function placeWidgets(widgets = [], groupIds = []) {
  const groups = new Set(groupIds);
  const out = { top: [], bottom: [], groups: {} };
  for (const w of widgets) {
    if (!w?.id || !w.type) continue;
    const area = w.area ?? "header";
    if (area === "header") out.top.push(w);
    else if (groups.has(area)) (out.groups[area] ??= []).push(w);
    else out.bottom.push(w);
  }
  return out;
}

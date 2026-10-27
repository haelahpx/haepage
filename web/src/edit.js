// Edit mode (IMPLEMENTATION.md M5.1): the config changes behind moving
// cards, groups and widgets. Each returns a new config and never changes
// the one it was given.
//
// Where things are kept:
//   - group order: `groups[].order`
//   - card order: `order` on the service (services you added) or on its
//     override (detected services); its group the same way
//   - widget order: the order of `widgets[]`, within each `area`
import { INBOX } from "./layout.js";

// Group and order of each service as the config says, applied over the
// list the server sent. A move shows at once, before the server has
// merged the change and sent the list again.
export function withLayout(list, config) {
  const manual = new Map(config.services.map((s) => [s.id, s]));
  return list.map((s) => {
    const m = manual.get(s.id);
    if (m) return { ...s, group: m.group, order: m.order };
    const o = config.overrides?.[s.id];
    if (o?.pinned && !o.hidden && o.group) return { ...s, group: o.group, order: o.order };
    return o?.order != null ? { ...s, order: o.order } : s;
  });
}

// Sorts by `order`; items without one keep their place after those with one.
export function byOrder(items) {
  return items
    .map((item, i) => [item, i])
    .sort(([a, i], [b, j]) => (a.order ?? Infinity) - (b.order ?? Infinity) || i - j)
    .map(([item]) => item);
}

const insert = (list, item, index) => {
  const out = list.filter((x) => x !== item);
  out.splice(Math.max(0, Math.min(index, out.length)), 0, item);
  return out;
};

// Groups that can be moved: everything but New and Other.
export const movableGroup = (section) => !section.inbox && section.id !== "";

// Whether a service may be dropped into a section. Detected services can't
// go to Other: without a group they would fall back to their suggested one.
export function canDropService(config, id, toGroup) {
  if (toGroup === INBOX) return false;
  if (toGroup === "") return config.services.some((s) => s.id === id);
  return true;
}

// Moves service `id` into section `toGroup` at `index` (counted without
// the moved service), renumbering that section's cards.
export function moveService(config, sections, id, toGroup, index) {
  if (!canDropService(config, id, toGroup)) return config;
  const target = sections.find((g) => g.id === toGroup)?.services.map((s) => s.id) ?? [];
  const ids = insert(target, id, index);
  const services = [...config.services];
  const overrides = { ...config.overrides };
  ids.forEach((sid, order) => {
    const i = services.findIndex((s) => s.id === sid);
    if (i >= 0) {
      const next = { ...services[i], order };
      if (sid === id) {
        if (toGroup) next.group = toGroup;
        else delete next.group;
      }
      services[i] = next;
      return;
    }
    const o = { ...overrides[sid], order };
    if (sid === id) Object.assign(o, { group: toGroup, pinned: true, hidden: false });
    overrides[sid] = o;
  });
  return { ...config, services, overrides };
}

// Moves group `id` to `index` among the movable sections, renumbering all
// groups. A section that only services name (no entry in `groups`) gets one.
export function moveGroup(config, sections, id, index) {
  const ids = insert(sections.filter(movableGroup).map((g) => g.id), id, index);
  const defined = new Map(config.groups.map((g) => [g.id, g]));
  const groups = ids.map((gid, order) => {
    const g = defined.get(gid) ?? { id: gid, name: sections.find((s) => s.id === gid)?.name ?? gid };
    return { ...g, order };
  });
  // Groups with no services aren't on the page; keep them, after the rest.
  for (const g of byOrder(config.groups)) {
    if (!ids.includes(g.id)) groups.push({ ...g, order: groups.length });
  }
  return { ...config, groups };
}

// The list a widget is shown in: "header" (top), a group id, or "sidebar"
// (bottom, also for groups that are gone). Same rules as placeWidgets.
export function widgetList(widget, groupIds) {
  const area = widget.area ?? "header";
  if (area === "header" || groupIds.includes(area)) return area;
  return "sidebar";
}

// Moves widget `id` into list `area` at `index` (counted without the moved
// widget) by its place in `widgets[]`.
export function moveWidget(config, groupIds, id, area, index) {
  const widget = config.widgets.find((w) => w.id === id);
  if (!widget) return config;
  const rest = config.widgets.filter((w) => w.id !== id);
  const inArea = rest.filter((w) => widgetList(w, groupIds) === area);
  const moved = { ...widget, area };
  const widgets = [...rest];
  if (index < inArea.length) widgets.splice(widgets.indexOf(inArea[Math.max(0, index)]), 0, moved);
  else if (inArea.length) widgets.splice(widgets.indexOf(inArea[inArea.length - 1]) + 1, 0, moved);
  else widgets.push(moved);
  return { ...config, widgets };
}

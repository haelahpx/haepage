// Turns groups and services into the sections shown on the page.

export const INBOX = "inbox";

const titleCase = (id) => id.replace(/[-_]+/g, " ").replace(/\b\p{L}/gu, (c) => c.toUpperCase());

// Order: the Inbox (newly detected services) first, then groups from the
// config in their order, then groups that services name but the config
// doesn't define (named from their id), then "Other" for services with no
// group. Empty sections are left out, and no service is ever dropped.
export function arrange(groups, services) {
  const defined = new Set(groups.map((g) => g.id));
  const of = (id) => services.filter((s) => s.group === id);
  const sections = [{ id: INBOX, name: "New", icon: "inbox", inbox: true, services: of(INBOX) }];

  for (const g of [...groups].sort((a, b) => (a.order ?? 0) - (b.order ?? 0))) {
    sections.push({ id: g.id, name: g.name, icon: g.icon, services: of(g.id) });
  }
  const extra = [];
  for (const s of services) {
    if (s.group && s.group !== INBOX && !defined.has(s.group) && !extra.includes(s.group)) extra.push(s.group);
  }
  for (const id of extra) sections.push({ id, name: titleCase(id), services: of(id) });
  sections.push({ id: "", name: "Other", services: services.filter((s) => !s.group) });

  return sections.filter((g) => g.services.length > 0);
}

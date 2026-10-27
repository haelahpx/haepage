// Live service list from the server's event stream (/api/events).

// Applies one server-sent event to the service list and returns the new
// list. Unknown events leave it unchanged.
export function applyEvent(list, type, data) {
  switch (type) {
    case "snapshot":
      return data.services ?? [];
    case "service.upsert": {
      const i = list.findIndex((s) => s.id === data.id);
      if (i < 0) return [...list, data];
      const next = list.slice();
      next[i] = data;
      return next;
    }
    case "service.remove":
      return list.filter((s) => s.id !== data.id);
    case "health":
      return list.map((s) => (s.id === data.id ? { ...s, health: data.health } : s));
    default:
      return list;
  }
}

export const SERVICE_EVENTS = ["snapshot", "service.upsert", "service.remove", "health"];

// Sends an override for a detected service (pin, hide, rename), or null to
// clear it.
export async function setOverride(id, override, fetchFn = fetch) {
  const res = await fetchFn(`./api/overrides/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(override),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
}

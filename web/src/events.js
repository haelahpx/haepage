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

export const LIVE_EVENTS = [...SERVICE_EVENTS, "integration", "integration.remove", "activity"];

const ACTIVITY_LIMIT = 50;

// Applies one server-sent event to the whole live state:
// { services, integrations: { [service]: state }, activity: [newest first] }.
export function applyLive(state, type, data) {
  switch (type) {
    case "snapshot":
      return {
        services: data.services ?? [],
        integrations: Object.fromEntries((data.integrations ?? []).map((i) => [i.service, i])),
        activity: [...(data.activity ?? [])].reverse().slice(0, ACTIVITY_LIMIT),
      };
    case "integration":
      return { ...state, integrations: { ...state.integrations, [data.service]: data } };
    case "integration.remove": {
      const { [data.service]: _gone, ...rest } = state.integrations;
      return { ...state, integrations: rest };
    }
    case "activity":
      return { ...state, activity: [data, ...state.activity].slice(0, ACTIVITY_LIMIT) };
    default:
      return { ...state, services: applyEvent(state.services, type, data) };
  }
}

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

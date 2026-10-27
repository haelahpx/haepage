import { emptyConfig } from "./defaults.js";
import { migrate } from "./migrate.js";

// Static mode keeps edits in the browser under this key until the user
// exports them (IMPLEMENTATION.md D10).
export const DRAFT_KEY = "haepage:config:v1";

// localStorage can be missing or throw (private windows, blocked storage).
export function browserStorage() {
  try {
    return globalThis.localStorage ?? null;
  } catch {
    return null;
  }
}

function readDraft(storage) {
  try {
    const draft = JSON.parse(storage?.getItem(DRAFT_KEY) ?? "null");
    if (!draft) return null;
    return { savedAt: Number(draft.savedAt) || 0, config: migrate(draft.config) };
  } catch {
    return null;
  }
}

// Loads the config: a browser draft newer than haepage.json wins, then the
// file, then built-in defaults. `error` explains a file that failed to load.
export async function loadConfig({ fetchFn = fetch, storage = browserStorage() } = {}) {
  const draft = readDraft(storage);
  let file = null;
  let fileTime = 0;
  let error = null;

  try {
    const res = await fetchFn("./haepage.json", { cache: "no-cache" });
    if (res.ok) {
      file = migrate(await res.json());
      fileTime = Date.parse(res.headers?.get("Last-Modified") ?? "") || 0;
    } else if (res.status !== 404) {
      error = `HTTP ${res.status}`;
    }
  } catch (err) {
    error = err.message;
  }

  if (draft && draft.savedAt > fileTime) return { config: draft.config, source: "draft", file, error };
  if (file) return { config: file, source: "file", file, error };
  return { config: emptyConfig(), source: "defaults", file: null, error };
}

export function saveDraft(config, storage = browserStorage(), now = Date.now()) {
  try {
    storage?.setItem(DRAFT_KEY, JSON.stringify({ savedAt: now, config }));
    return true;
  } catch {
    return false;
  }
}

export function clearDraft(storage = browserStorage()) {
  try {
    storage?.removeItem(DRAFT_KEY);
  } catch {
    // Nothing to clear when storage is unavailable.
  }
}

// Thrown when the server's config changed since it was loaded (HTTP 412).
export class ConflictError extends Error {}

// Server mode: the config lives on the server, versioned by ETag.
export async function loadServerConfig(fetchFn = fetch) {
  const res = await fetchFn("./api/config", { cache: "no-store" });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const etag = res.headers.get("ETag");
  return { config: migrate(await res.json()), etag, source: "server", file: null, error: null };
}

// Saves the whole config. etag must be the one last loaded or saved.
export async function saveServerConfig(config, etag, fetchFn = fetch) {
  const res = await fetchFn("./api/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json", "If-Match": etag ?? "" },
    body: JSON.stringify(config),
  });
  if (res.status === 412) throw new ConflictError("The config was changed somewhere else.");
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error ?? `HTTP ${res.status}`);
  return res.headers.get("ETag") ?? body.etag;
}

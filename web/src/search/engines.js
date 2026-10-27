// Web search engines and bang parsing (IMPLEMENTATION.md D7): "!g cats"
// or "cats !g" searches the engine whose prefix is "g".

export const BUILTIN_ENGINES = [
  { id: "duckduckgo", name: "DuckDuckGo", prefix: "d", url: "https://duckduckgo.com/?q=%s" },
  { id: "google", name: "Google", prefix: "g", url: "https://www.google.com/search?q=%s" },
  { id: "brave", name: "Brave Search", prefix: "b", url: "https://search.brave.com/search?q=%s" },
  { id: "wikipedia", name: "Wikipedia", prefix: "w", url: "https://en.wikipedia.org/w/index.php?search=%s" },
  { id: "youtube", name: "YouTube", prefix: "yt", url: "https://www.youtube.com/results?search_query=%s" },
  { id: "github", name: "GitHub", prefix: "gh", url: "https://github.com/search?q=%s" },
];

// Built-in engines with config engines added, or replacing a built-in by id.
export function allEngines(custom = []) {
  const byId = new Map(BUILTIN_ENGINES.map((e) => [e.id, e]));
  for (const engine of custom) {
    if (engine?.id && engine.url?.includes("%s")) {
      byId.set(engine.id, { ...byId.get(engine.id), ...engine, name: engine.name ?? byId.get(engine.id)?.name ?? engine.id });
    }
  }
  return [...byId.values()];
}

export function findEngine(engines, id) {
  return engines.find((e) => e.id === id) ?? engines[0];
}

// Splits a query into its bang engine (if any) and search terms.
export function parseQuery(input, engines) {
  const text = input.trim();
  const leading = text.match(/^!(\S+)(?:\s+([\s\S]*))?$/);
  const trailing = text.match(/^([\s\S]*?)\s+!(\S+)$/);
  const [prefix, terms] = leading ? [leading[1], leading[2] ?? ""] : trailing ? [trailing[2], trailing[1]] : [];
  if (prefix) {
    const engine = engines.find((e) => e.prefix === prefix.toLowerCase());
    if (engine) return { engine, terms: terms.trim(), bang: true };
  }
  return { engine: null, terms: text, bang: false };
}

// The URL to open for a search. Empty terms open the engine's home page.
export function searchUrl(engine, terms) {
  if (!terms) return new URL(engine.url.replace("%s", "")).origin;
  return engine.url.replace("%s", encodeURIComponent(terms));
}

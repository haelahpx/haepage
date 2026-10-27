import { browserStorage } from "../config/store.js";

// index.html inlines this theme's CSS in <style id="hp-theme">, so the
// default theme paints with no extra request.
const INLINE_THEME = "tonal";
const CACHE_KEY = "haepage:theme-css:v1";

export const BUILTIN_THEMES = [
  { id: "tonal", name: "Tonal" },
  { id: "glass", name: "Glass" },
];

const root = () => document.documentElement;

export function isThemeUrl(theme) {
  return /^https:\/\//i.test(theme);
}

// The theme offered by a shared link (?theme=https://…), or null.
export function themeFromLink(search) {
  const theme = new URLSearchParams(search).get("theme");
  return theme && isThemeUrl(theme) ? theme : null;
}

// A link that offers `theme` to whoever opens this page.
export function shareLink(theme, loc = globalThis.location) {
  return `${loc.origin}${loc.pathname}?theme=${encodeURIComponent(theme)}`;
}

async function errorOf(res) {
  try {
    return (await res.json()).error ?? `HTTP ${res.status}`;
  } catch {
    return `HTTP ${res.status}`;
  }
}

// Server mode: the server fetches the link through its SSRF guard and keeps
// a pinned copy. `refresh` fetches the link again.
export async function importTheme(url, { refresh = false, fetchFn = fetch } = {}) {
  const res = await fetchFn("./api/themes/import", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, refresh }),
  });
  if (!res.ok) throw new Error(`The theme could not be imported: ${await errorOf(res)}`);
  return res.json();
}

// Asks the server to check CSS against the theme budgets. `target` is
// {url} for an imported theme or {css} for custom CSS. Resolves to a list
// of {rule, severity, line, message}.
export async function lintCss(target, { fetchFn = fetch } = {}) {
  const res = await fetchFn("./api/themes/lint", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(target),
  });
  if (!res.ok) throw new Error(await errorOf(res));
  return res.json();
}

// Fetches theme CSS. Link themes go through the server when it can import
// them (importing on first use), and straight from the browser otherwise,
// which needs the host to allow CORS.
export async function fetchThemeCss(theme, { server = false, fetchFn = fetch } = {}) {
  if (isThemeUrl(theme) && server) {
    const remote = () => fetchFn(`./api/themes/remote?url=${encodeURIComponent(theme)}`);
    let res = await remote();
    if (res.status === 404) {
      await importTheme(theme, { fetchFn });
      res = await remote();
    }
    if (!res.ok) throw new Error(`Theme "${theme}" could not be loaded: ${await errorOf(res)}`);
    return res.text();
  }
  let res;
  try {
    res = await fetchFn(themeHref(theme));
  } catch {
    throw new Error(`Theme "${theme}" could not be loaded. The site may not allow other pages to use its files (CORS).`);
  }
  if (!res.ok) throw new Error(`Theme "${theme}" could not be loaded (HTTP ${res.status})`);
  return res.text();
}

function themeHref(theme) {
  if (isThemeUrl(theme)) return theme;
  if (!/^[a-z0-9-]+$/.test(theme)) throw new Error(`Unknown theme "${theme}"`);
  return `./themes/${theme}.css`;
}

function setThemeCss(css) {
  document.getElementById("hp-theme").textContent = css;
}

// Re-applies the last theme from the browser cache before the config
// loads, so a non-default theme doesn't flash the default first.
export function applyCachedTheme() {
  try {
    const cached = JSON.parse(browserStorage()?.getItem(CACHE_KEY) ?? "null");
    if (cached?.theme && cached.theme !== INLINE_THEME) {
      setThemeCss(cached.css);
      root().dataset.theme = cached.theme;
    }
  } catch {
    // A bad cache only costs a flash of the default theme.
  }
}

// Loads and applies a theme by id or https:// URL. Throws when it can't be
// fetched, leaving the current theme in place. `force` reloads the theme
// that is already applied (after an update).
export async function applyTheme(theme, { server = false, force = false } = {}) {
  if (!force && root().dataset.theme === theme) return;
  const css = await fetchThemeCss(theme, { server });
  setThemeCss(css);
  root().dataset.theme = theme;
  try {
    browserStorage()?.setItem(CACHE_KEY, JSON.stringify({ theme, css }));
  } catch {
    // Caching is optional.
  }
}

export function applyCustomCss(css) {
  document.getElementById("hp-custom").textContent = css ?? "";
}

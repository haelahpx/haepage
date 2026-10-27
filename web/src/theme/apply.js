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
// fetched, leaving the current theme in place.
export async function applyTheme(theme) {
  if (root().dataset.theme === theme) return;
  const res = await fetch(themeHref(theme));
  if (!res.ok) throw new Error(`Theme "${theme}" could not be loaded (HTTP ${res.status})`);
  const css = await res.text();
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

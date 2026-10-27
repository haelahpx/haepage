// Picks an effect tier for settings.fx = "auto". Low-end or data-saving
// devices get "lite"; everything else gets "balanced". "full" is opt-in
// until frame-time measurement lands (IMPLEMENTATION.md §6.6, M3.3).
export function autoFx(nav = globalThis.navigator, reducedMotion = false) {
  if (reducedMotion || nav?.connection?.saveData) return "lite";
  if (nav?.deviceMemory && nav.deviceMemory <= 2) return "lite";
  if (nav?.hardwareConcurrency && nav.hardwareConcurrency <= 2) return "lite";
  return "balanced";
}

export function resolveFx(setting, nav, reducedMotion) {
  return ["full", "balanced", "lite"].includes(setting) ? setting : autoFx(nav, reducedMotion);
}

export function resolveScheme(setting, prefersDark) {
  return setting === "light" || setting === "dark" ? setting : prefersDark ? "dark" : "light";
}

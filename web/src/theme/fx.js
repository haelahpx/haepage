// Effect tier for settings.fx = "auto" (IMPLEMENTATION.md §6.6). Low-end or
// data-saving devices get "lite" straight away; others are measured once by
// frame timing, and the result is kept per device in localStorage.

export const FX_KEY = "haepage:fx:v1";
export const FX_FRAMES = 20;
const TIERS = ["full", "balanced", "lite"];

// Devices that get "lite" without measuring.
export function hardLite(nav = globalThis.navigator, reducedMotion = false) {
  if (reducedMotion || nav?.connection?.saveData) return true;
  if (nav?.deviceMemory && nav.deviceMemory <= 2) return true;
  if (nav?.hardwareConcurrency && nav.hardwareConcurrency <= 2) return true;
  return false;
}

// Median frame interval → tier. The cut-offs sit above a 60 Hz frame
// (16.7 ms), so a healthy 60 Hz screen lands on "full" (decision D14).
export function tierFromFrames(intervals) {
  if (!intervals?.length) return null;
  const sorted = [...intervals].sort((a, b) => a - b);
  const mid = sorted.length >> 1;
  const median = sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
  if (median > 25) return "lite";
  if (median > 18) return "balanced";
  return "full";
}

// Resolves with `count` requestAnimationFrame intervals in ms.
export function measureFrames(count = FX_FRAMES, raf = globalThis.requestAnimationFrame) {
  return new Promise((resolve) => {
    const intervals = [];
    let last = null;
    const tick = (t) => {
      if (last !== null) intervals.push(t - last);
      last = t;
      if (intervals.length < count) raf(tick);
      else resolve(intervals);
    };
    raf(tick);
  });
}

export function readMeasuredFx(storage) {
  try {
    const tier = storage?.getItem(FX_KEY);
    return TIERS.includes(tier) ? tier : null;
  } catch {
    return null;
  }
}

export function saveMeasuredFx(storage, tier) {
  try {
    if (tier) storage?.setItem(FX_KEY, tier);
    else storage?.removeItem(FX_KEY);
  } catch {
    // Storage blocked: the device is measured again on the next load.
  }
}

// `measured` is the stored tier for this device, or null before measuring.
export function autoFx(nav = globalThis.navigator, reducedMotion = false, measured = null) {
  if (hardLite(nav, reducedMotion)) return "lite";
  return measured ?? "balanced";
}

export function resolveFx(setting, nav, reducedMotion, measured) {
  return TIERS.includes(setting) ? setting : autoFx(nav, reducedMotion, measured);
}

export function resolveScheme(setting, prefersDark) {
  return setting === "light" || setting === "dark" ? setting : prefersDark ? "dark" : "light";
}

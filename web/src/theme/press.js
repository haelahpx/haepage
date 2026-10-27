// Sets --press-x and --press-y on the pressed control, so themes can start
// a ripple where the pointer touched (docs/theme-api.md). One passive
// listener for the whole page; no work happens while nothing is pressed.
export const PRESSABLE = ".hp-card, .hp-button, .hp-icon-button, .hp-result";

export function pressPoint(rect, x, y) {
  return { x: `${Math.round(x - rect.left)}px`, y: `${Math.round(y - rect.top)}px` };
}

export function trackPresses(doc = document) {
  const onDown = (e) => {
    const el = e.target.closest?.(PRESSABLE);
    if (!el) return;
    const p = pressPoint(el.getBoundingClientRect(), e.clientX, e.clientY);
    el.style.setProperty("--press-x", p.x);
    el.style.setProperty("--press-y", p.y);
  };
  doc.addEventListener("pointerdown", onDown, { passive: true });
  return () => doc.removeEventListener("pointerdown", onDown);
}

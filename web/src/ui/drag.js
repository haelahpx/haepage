// Drag and drop for edit mode, with pointer events and no library.
//
// Markup contract:
//   - something you can move: data-edit="service|group|widget", data-id,
//     data-list (the list it sits in: a group id, "groups", or a widget area)
//   - somewhere to drop: data-zone-<kind>="<list> [<list>…]" (the nearest
//     zone around the pointer picks which lists are candidates)
//
// A mouse drags at once; touch and pen drag after a long press, so a quick
// swipe still scrolls the page. Esc cancels a drag.
import { useEffect, useRef } from "preact/hooks";

const MOUSE_SLOP = 5;
const TOUCH_SLOP = 8;
const LONG_PRESS = 350;
const CONTROLS = "input, textarea, select, button, [contenteditable], .hp-widget-body a";

// Where an item dropped at (x, y) would go: { list, index, rect, axis }, or
// null when the pointer isn't over a zone for this kind.
export function dropTarget(kind, dragged, x, y) {
  const zone = document.elementFromPoint(x, y)?.closest(`[data-zone-${kind}]`);
  if (!zone) return null;
  const lists = zone.getAttribute(`data-zone-${kind}`).split(" ");
  const items = [...document.querySelectorAll(`[data-edit="${kind}"]`)].filter(
    (el) => el !== dragged && lists.includes(el.dataset.list),
  );
  if (items.length === 0) return { list: lists[0], index: 0, rect: zone.getBoundingClientRect(), axis: "inside" };

  let best = null;
  for (const el of items) {
    const r = el.getBoundingClientRect();
    const dx = Math.max(r.left - x, 0, x - r.right);
    const dy = Math.max(r.top - y, 0, y - r.bottom);
    const d = dx * dx + dy * dy;
    if (!best || d < best.d) best = { el, r, d };
  }
  const { el, r } = best;
  const sameList = items.filter((i) => i.dataset.list === el.dataset.list);
  // In a row (other items beside it), drop beside the item when the pointer
  // is level with it; in a column, always above or below.
  const inRow = sameList.some((i) => {
    const o = i.getBoundingClientRect();
    return i !== el && o.top < r.bottom && o.bottom > r.top;
  });
  const level = inRow && y >= r.top && y <= r.bottom;
  const after = level ? x > r.left + r.width / 2 : y > r.top + r.height / 2;
  return {
    list: el.dataset.list,
    index: sameList.indexOf(el) + (after ? 1 : 0),
    rect: r,
    axis: level ? (after ? "right" : "left") : after ? "bottom" : "top",
  };
}

function placeMarker(marker, target) {
  if (!target) {
    marker.hidden = true;
    return;
  }
  const { rect: r, axis } = target;
  const t = 4;
  const box = {
    left: { left: r.left - t - 2, top: r.top, width: t, height: r.height },
    right: { left: r.right + 2, top: r.top, width: t, height: r.height },
    top: { left: r.left, top: r.top - t - 2, width: r.width, height: t },
    bottom: { left: r.left, top: r.bottom + 2, width: r.width, height: t },
    inside: { left: r.left, top: r.top, width: r.width, height: r.height },
  }[axis];
  marker.hidden = false;
  marker.dataset.axis = axis;
  Object.assign(marker.style, {
    transform: `translate(${box.left}px, ${box.top}px)`,
    width: `${box.width}px`,
    height: `${box.height}px`,
  });
}

// Turns on dragging while `enabled`. onDrop({ kind, id, list, index }).
export function useDrag(enabled, onDrop) {
  const handler = useRef(onDrop);
  handler.current = onDrop;

  useEffect(() => {
    if (!enabled) return undefined;
    const marker = Object.assign(document.createElement("div"), { className: "hp-drop-marker", hidden: true });
    document.body.append(marker);
    let press = null; // { el, kind, x0, y0, id, timer, dragging, target }

    const start = () => {
      press.dragging = true;
      press.el.classList.add("hp-dragging");
      document.documentElement.dataset.dragging = "";
      navigator.vibrate?.(10);
    };
    const end = (drop) => {
      if (!press) return;
      clearTimeout(press.timer);
      const { el, kind, dragging, target } = press;
      press = null;
      marker.hidden = true;
      if (!dragging) return;
      el.classList.remove("hp-dragging");
      el.style.translate = "";
      delete document.documentElement.dataset.dragging;
      // The click that follows a drag must not open the card.
      window.addEventListener("click", swallow, true);
      setTimeout(() => window.removeEventListener("click", swallow, true), 0);
      if (drop && target) handler.current({ kind, id: el.dataset.id, list: target.list, index: target.index });
    };
    const swallow = (e) => {
      e.preventDefault();
      e.stopPropagation();
    };

    const onDown = (e) => {
      if (press || e.button !== 0 || e.target.closest(CONTROLS)) return;
      const el = e.target.closest("[data-edit]");
      if (!el) return;
      press = { el, kind: el.dataset.edit, x0: e.clientX, y0: e.clientY, pointerId: e.pointerId, touch: e.pointerType !== "mouse" };
      if (press.touch) press.timer = setTimeout(() => press && start(), LONG_PRESS);
    };
    const onMove = (e) => {
      if (!press || e.pointerId !== press.pointerId) return;
      const dx = e.clientX - press.x0;
      const dy = e.clientY - press.y0;
      if (!press.dragging) {
        const far = Math.hypot(dx, dy) > (press.touch ? TOUCH_SLOP : MOUSE_SLOP);
        if (press.touch && far) return end(false); // a swipe: let the page scroll
        if (!press.touch && far) start();
        else return;
      }
      press.el.style.translate = `${dx}px ${dy}px`;
      press.target = dropTarget(press.kind, press.el, e.clientX, e.clientY);
      placeMarker(marker, press.target);
    };
    const onUp = (e) => press && e.pointerId === press.pointerId && end(true);
    const onCancel = (e) => press && e.pointerId === press.pointerId && end(false);
    const onKey = (e) => e.key === "Escape" && press?.dragging && (e.stopPropagation(), end(false));
    // Once a touch drag has started, the finger moves the item, not the page.
    const onTouchMove = (e) => press?.dragging && e.cancelable && e.preventDefault();
    const onContext = (e) => press && e.preventDefault();
    // Cards are links: stop the browser's own link dragging.
    const onNativeDrag = (e) => e.target.closest?.("[data-edit]") && e.preventDefault();

    document.addEventListener("pointerdown", onDown);
    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
    document.addEventListener("pointercancel", onCancel);
    document.addEventListener("keydown", onKey, true);
    document.addEventListener("touchmove", onTouchMove, { passive: false });
    document.addEventListener("contextmenu", onContext);
    document.addEventListener("dragstart", onNativeDrag);
    return () => {
      end(false);
      marker.remove();
      document.removeEventListener("pointerdown", onDown);
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
      document.removeEventListener("pointercancel", onCancel);
      document.removeEventListener("keydown", onKey, true);
      document.removeEventListener("touchmove", onTouchMove, { passive: false });
      document.removeEventListener("contextmenu", onContext);
      document.removeEventListener("dragstart", onNativeDrag);
    };
  }, [enabled]);
}

import { useCallback, useRef, useState } from "preact/hooks";

const SHOW_MS = 6000;
const MAX = 3;

// Small pop-ups in the corner for new activity. Each disappears after a few
// seconds; hovering keeps it open.
export function useToasts() {
  const [toasts, setToasts] = useState([]);
  const timers = useRef(new Map());

  const dismiss = useCallback((id) => {
    clearTimeout(timers.current.get(id));
    timers.current.delete(id);
    setToasts((list) => list.filter((t) => t.id !== id));
  }, []);

  const schedule = useCallback((id) => timers.current.set(id, setTimeout(() => dismiss(id), SHOW_MS)), [dismiss]);

  const push = useCallback(
    (toast) => {
      setToasts((list) => [toast, ...list.filter((t) => t.id !== toast.id)].slice(0, MAX));
      schedule(toast.id);
    },
    [schedule],
  );

  const hold = useCallback((id) => clearTimeout(timers.current.get(id)), []);

  return { toasts, push, dismiss, hold, release: schedule };
}

export function Toasts({ toasts, dismiss, hold, release }) {
  if (toasts.length === 0) return null;
  return (
    <div class="hp-toasts" role="status" aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} class="hp-toast" onMouseEnter={() => hold(t.id)} onMouseLeave={() => release(t.id)}>
          {t.image && <img class="hp-toast-image" src={t.image} alt="" />}
          <div class="hp-toast-body">
            <p class="hp-toast-title">{t.title}</p>
            {t.detail && <p class="hp-toast-detail">{t.detail}</p>}
          </div>
          <button type="button" class="hp-icon-button" onClick={() => dismiss(t.id)} aria-label="Dismiss">
            ✕
          </button>
        </div>
      ))}
    </div>
  );
}

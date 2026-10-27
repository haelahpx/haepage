import { useEffect, useState } from "preact/hooks";

const timeFormat = new Intl.DateTimeFormat([], { hour: "2-digit", minute: "2-digit" });

export function useNow(intervalMs = 15_000) {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}

export function formatDate(now) {
  return now.toLocaleDateString([], { weekday: "long", month: "long", day: "numeric" });
}

// The time split into parts, so themes can lay hours and minutes out
// differently.
export function ClockTime({ now }) {
  const parts = Object.fromEntries(timeFormat.formatToParts(now).map((p) => [p.type, p.value]));
  return (
    <time class="hp-clock-time" dateTime={now.toISOString()}>
      <span class="hp-clock-h">{parts.hour}</span>
      <span class="hp-clock-sep">:</span>
      <span class="hp-clock-m">{parts.minute}</span>
      {parts.dayPeriod && <span class="hp-clock-ampm">{parts.dayPeriod}</span>}
    </time>
  );
}

export function SettingsButton({ onClick }) {
  return (
    <button type="button" class="hp-icon-button" onClick={onClick} aria-label="Settings" title="Settings">
      <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
        <path
          fill="currentColor"
          d="M19.4 13a7.5 7.5 0 0 0 0-2l2.1-1.6-2-3.5-2.5 1a7.6 7.6 0 0 0-1.7-1L15 3h-4l-.4 2.9a7.6 7.6 0 0 0-1.7 1l-2.5-1-2 3.5L6.6 11a7.5 7.5 0 0 0 0 2l-2.1 1.6 2 3.5 2.5-1c.5.4 1.1.7 1.7 1L11 21h4l.4-2.9c.6-.3 1.2-.6 1.7-1l2.5 1 2-3.5-2.2-1.6ZM13 15.5a3.5 3.5 0 1 1 0-7 3.5 3.5 0 0 1 0 7Z"
        />
      </svg>
    </button>
  );
}

// Turns edit mode (moving cards, groups and widgets) on and off.
export function EditButton({ editing, onClick }) {
  return (
    <button
      type="button"
      class={editing ? "hp-icon-button hp-edit-button hp-edit-button--on" : "hp-icon-button hp-edit-button"}
      onClick={onClick}
      aria-pressed={editing ? "true" : "false"}
      aria-label="Edit layout"
      title={editing ? "Done editing" : "Edit layout"}
    >
      <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
        <path fill="currentColor" d="M4 16.5V20h3.5L18 9.5 14.5 6 4 16.5Zm11.6-11.6 3.5 3.5 1.2-1.2a1.5 1.5 0 0 0 0-2.1l-1.4-1.4a1.5 1.5 0 0 0-2.1 0l-1.2 1.2Z" />
      </svg>
    </button>
  );
}

// Header for the simple layout. The bento layout puts these in its hero tile.
export function Header({ title, onSettings, editing, onEdit }) {
  const now = useNow();
  return (
    <header class="hp-header">
      <h1 class="hp-title">{title}</h1>
      <div class="hp-header-end">
        <div class="hp-clock">
          <ClockTime now={now} />
          <span class="hp-clock-date">{formatDate(now)}</span>
        </div>
        {onEdit && <EditButton editing={editing} onClick={onEdit} />}
        <SettingsButton onClick={onSettings} />
      </div>
    </header>
  );
}

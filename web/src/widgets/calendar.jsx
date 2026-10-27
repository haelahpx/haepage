import { useState } from "preact/hooks";
import { useWidgetData } from "./api.js";
import { groupByDay } from "./lib.js";
import { NeedsServer, SetupForm } from "./setup.jsx";

const time = (iso) => new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" }).format(Date.parse(iso));

// Upcoming events from an iCalendar (.ics) link, grouped by day.
// options: url, days (14), limit (10).
export default function Calendar({ widget, options, setOptions, ctx }) {
  const can = ctx.capabilities?.has("widgets.proxy");
  const [editing, setEditing] = useState(!options.url);
  const { data, error } = useWidgetData(widget, can && !!options.url, JSON.stringify([options.url, options.days, options.limit]), widget.refresh ?? 900);

  if (!can) return <NeedsServer />;
  if (editing) {
    return (
      <SetupForm
        label="Calendar link (.ics)"
        placeholder="https://calendar.example.com/private/basic.ics"
        help="Most calendar apps offer a private iCal link. Put its secret part in Settings → Secrets and write {{secrets.name}} here to keep it off browsers."
        initial={options.url}
        onSave={(url) => {
          setOptions({ url });
          setEditing(false);
        }}
        onCancel={options.url && (() => setEditing(false))}
      />
    );
  }
  const groups = data ? groupByDay(data.events) : [];
  return (
    <div class="hp-w-calendar">
      {error && <p class="hp-widget-empty" role="alert">{error}</p>}
      {data === undefined && !error && <p class="hp-widget-empty">Loading…</p>}
      {data && groups.length === 0 && <p class="hp-widget-empty">Nothing in the next {options.days ?? 14} days.</p>}
      {groups.map((g) => (
        <section key={g.label}>
          <h3>{g.label}</h3>
          <ul>
            {g.events.map((e, i) => (
              <li key={i}>
                <span class="hp-w-calendar-time">{e.allDay ? "All day" : time(e.start)}</span>
                <span class="hp-w-calendar-what">
                  {e.title || "(no title)"}
                  {e.location && <small>{e.location}</small>}
                </span>
              </li>
            ))}
          </ul>
        </section>
      ))}
      <button type="button" class="hp-link-button hp-w-api-edit" onClick={() => setEditing(true)}>
        Change calendar
      </button>
    </div>
  );
}

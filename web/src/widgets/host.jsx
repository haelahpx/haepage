import { useEffect, useErrorBoundary, useState } from "preact/hooks";
import { WIDGET_TYPES } from "./registry.js";

// Loaded modules, so a second widget of the same type renders at once.
const loaded = {};

// Renders the widget's module. A crash is caught here, so one broken
// widget never takes the page down.
function Body({ widget, ctx, onChange }) {
  const [error, reset] = useErrorBoundary();
  const type = WIDGET_TYPES[widget.type];
  const [mod, setMod] = useState(() => loaded[widget.type] ?? null);
  const [loadError, setLoadError] = useState(null);

  useEffect(() => {
    if (mod || !type) return;
    type
      .load()
      .then((m) => {
        loaded[widget.type] = m;
        setMod(m);
      })
      .catch((err) => setLoadError(err));
  }, [widget.type]);

  if (!type) return <p class="hp-widget-empty">Unknown widget type "{widget.type}".</p>;
  if (error || loadError) {
    return (
      <p class="hp-widget-empty" role="alert">
        This widget stopped working{error?.message ? ` (${error.message})` : ""}.{" "}
        <button
          type="button"
          class="hp-link-button"
          onClick={() => {
            setLoadError(null);
            reset();
          }}
        >
          Try again
        </button>
      </p>
    );
  }
  if (!mod) return <p class="hp-widget-empty" aria-busy="true" />;

  const Widget = mod.default;
  const options = widget.options ?? {};
  const setOptions = (patch) => onChange({ ...widget, options: { ...options, ...patch } });
  return <Widget widget={widget} options={options} setOptions={setOptions} ctx={ctx} />;
}

// One widget: a tile in Bento, a block in Simple. `onChange` saves the
// widget back into haepage.json (Server mode: for everyone). With `list`
// set (edit mode), it can be dragged within and between widget areas.
export function WidgetTile({ widget, ctx, onChange, inline = false, list = null }) {
  const type = WIDGET_TYPES[widget.type];
  const title = widget.title ?? type?.name ?? widget.type;
  const cls = `hp-widget hp-widget--${widget.type}${inline ? "" : " hp-tile"}`;
  const edit = list === null ? {} : { "data-edit": "widget", "data-id": widget.id, "data-list": list, tabIndex: 0 };
  return (
    <section class={cls} aria-label={title} {...edit}>
      {widget.title !== "" && <h2 class="hp-widget-title">{title}</h2>}
      <div class="hp-widget-body">
        <Body widget={widget} ctx={ctx} onChange={onChange} />
      </div>
    </section>
  );
}

import { useState } from "preact/hooks";
import { useWidgetData } from "./api.js";
import { arrayPaths, formatValue, getPath, leafPaths } from "./lib.js";

// The no-code API widget (IMPLEMENTATION.md M4.4). The server fetches the
// address (so {{secrets.x}} in it or in headers stays on the server) and
// this module picks fields out of the JSON answer.
//
// options: url, headers {name: value}, display ("stat", "list", "progress",
// "table"), items (path to an array, for list and table), fields
// [{label, path, format, max}] (paths are relative to each item in a list
// or table; max is a number or a path, for progress).

const FORMATS = ["auto", "number", "percent", "bytes", "duration", "date", "text"];
const DISPLAYS = { stat: "Numbers", progress: "Progress bars", list: "List", table: "Table" };
const ROWS = 8;

function maxOf(field, item) {
  if (field.max === undefined || field.max === "") return 100;
  const n = Number(field.max);
  return Number.isFinite(n) ? n : Number(getPath(item, field.max));
}

function View({ data, options }) {
  const fields = options.fields ?? [];
  if (fields.length === 0) return <p class="hp-widget-empty">No fields yet. Choose Edit to pick some.</p>;

  if (options.display === "list" || options.display === "table") {
    const items = getPath(data, options.items);
    if (!Array.isArray(items)) return <p class="hp-widget-empty">"{options.items || "(whole answer)"}" isn't a list.</p>;
    const rows = items.slice(0, ROWS);
    if (options.display === "table") {
      return (
        <div class="hp-w-api-table">
          <table>
            <thead>
              <tr>
                {fields.map((f) => (
                  <th key={f.path}>{f.label || f.path}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((item, i) => (
                <tr key={i}>
                  {fields.map((f) => (
                    <td key={f.path}>{formatValue(getPath(item, f.path), f.format)}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
    }
    const [first, ...rest] = fields;
    return (
      <ul class="hp-w-api-list">
        {rows.map((item, i) => (
          <li key={i}>
            <span>{formatValue(getPath(item, first.path), first.format)}</span>
            <span class="hp-w-api-muted">{rest.map((f) => formatValue(getPath(item, f.path), f.format)).join(" · ")}</span>
          </li>
        ))}
      </ul>
    );
  }

  if (options.display === "progress") {
    return (
      <ul class="hp-w-system">
        {fields.map((f, i) => {
          const value = Number(getPath(data, f.path));
          const max = maxOf(f, data);
          const pct = max > 0 && Number.isFinite(value) ? Math.max(0, Math.min(100, (value / max) * 100)) : 0;
          return (
            <li key={f.path}>
              <span class="hp-w-system-top">
                <span>{f.label || f.path}</span>
                <strong>{formatValue(value, f.format)}</strong>
              </span>
              <span class="hp-bar" style={{ "--avatar-hue": (211 + i * 67) % 360 }}>
                <span class="hp-bar-fill" style={{ "--pct": `${pct}%` }} />
              </span>
            </li>
          );
        })}
      </ul>
    );
  }

  return (
    <dl class="hp-w-api-stats">
      {fields.map((f) => (
        <div key={f.path}>
          <dd>{formatValue(getPath(data, f.path), f.format)}</dd>
          <dt>{f.label || f.path}</dt>
        </div>
      ))}
    </dl>
  );
}

// Setup: address, headers, how to show it, and fields. When an answer has
// arrived, its fields are offered as one-click suggestions.
function Editor({ options, setOptions, data, onDone }) {
  const [draft, setDraft] = useState(() => ({
    url: options.url ?? "",
    headers: Object.entries(options.headers ?? {}).map(([name, value]) => ({ name, value })),
    display: options.display ?? "stat",
    items: options.items ?? "",
    fields: options.fields ?? [],
  }));
  const set = (patch) => setDraft((d) => ({ ...d, ...patch }));
  const listy = draft.display === "list" || draft.display === "table";
  const scope = listy ? getPath(data, draft.items)?.[0] : data;
  const suggestions = data !== undefined ? leafPaths(scope ?? {}).filter((p) => !draft.fields.some((f) => f.path === p.path)) : [];
  const setField = (i, patch) => set({ fields: draft.fields.map((f, j) => (j === i ? { ...f, ...patch } : f)) });

  return (
    <form
      class="hp-w-api-editor"
      onSubmit={(e) => {
        e.preventDefault();
        setOptions({
          url: draft.url.trim(),
          headers: Object.fromEntries(draft.headers.filter((h) => h.name.trim()).map((h) => [h.name.trim(), h.value])),
          display: draft.display,
          items: listy ? draft.items.trim() : undefined,
          fields: draft.fields.filter((f) => f.path.trim() || listy),
        });
        // With no fields yet, stay open: the answer's fields are offered here.
        if (draft.fields.length > 0) onDone();
      }}
    >
      <label>
        <span>Address</span>
        <input
          placeholder="http://192.168.1.2/api/stats"
          value={draft.url}
          onInput={(e) => set({ url: e.currentTarget.value })}
          required
        />
      </label>

      <fieldset>
        <legend>
          Headers <small>Put keys in Settings → Secrets and write {"{{secrets.name}}"} here.</small>
        </legend>
        {draft.headers.map((h, i) => (
          <div class="hp-w-api-row" key={i}>
            <input
              placeholder="Authorization"
              aria-label="Header name"
              value={h.name}
              onInput={(e) => set({ headers: draft.headers.map((x, j) => (j === i ? { ...x, name: e.currentTarget.value } : x)) })}
            />
            <input
              placeholder="Bearer {{secrets.name}}"
              aria-label="Header value"
              value={h.value}
              onInput={(e) => set({ headers: draft.headers.map((x, j) => (j === i ? { ...x, value: e.currentTarget.value } : x)) })}
            />
            <button type="button" class="hp-w-remove" aria-label="Remove header" onClick={() => set({ headers: draft.headers.filter((_, j) => j !== i) })}>
              ×
            </button>
          </div>
        ))}
        <button type="button" class="hp-link-button" onClick={() => set({ headers: [...draft.headers, { name: "", value: "" }] })}>
          Add a header
        </button>
      </fieldset>

      <label>
        <span>Show as</span>
        <select value={draft.display} onChange={(e) => set({ display: e.currentTarget.value })}>
          {Object.entries(DISPLAYS).map(([k, v]) => (
            <option value={k}>{v}</option>
          ))}
        </select>
      </label>

      {listy && (
        <label>
          <span>List in the answer</span>
          {data !== undefined && arrayPaths(data).length > 0 ? (
            <select value={draft.items} onChange={(e) => set({ items: e.currentTarget.value })}>
              <option value="">Choose…</option>
              {arrayPaths(data).map((p) => (
                <option value={p}>{p || "(the whole answer)"}</option>
              ))}
            </select>
          ) : (
            <input placeholder="data.items" value={draft.items} onInput={(e) => set({ items: e.currentTarget.value })} />
          )}
        </label>
      )}

      <fieldset>
        <legend>Fields</legend>
        {draft.fields.map((f, i) => (
          <div class="hp-w-api-row" key={i}>
            <input placeholder="Label" aria-label="Field label" value={f.label ?? ""} onInput={(e) => setField(i, { label: e.currentTarget.value })} />
            <input placeholder="stats.blocked" aria-label="Field path" value={f.path} onInput={(e) => setField(i, { path: e.currentTarget.value })} />
            <select aria-label="Format" value={f.format ?? "auto"} onChange={(e) => setField(i, { format: e.currentTarget.value })}>
              {FORMATS.map((x) => (
                <option value={x}>{x}</option>
              ))}
            </select>
            {draft.display === "progress" && (
              <input placeholder="Max (100)" aria-label="Maximum" value={f.max ?? ""} onInput={(e) => setField(i, { max: e.currentTarget.value })} />
            )}
            <button type="button" class="hp-w-remove" aria-label="Remove field" onClick={() => set({ fields: draft.fields.filter((_, j) => j !== i) })}>
              ×
            </button>
          </div>
        ))}
        <button type="button" class="hp-link-button" onClick={() => set({ fields: [...draft.fields, { label: "", path: "", format: "auto" }] })}>
          Add a field
        </button>
        {suggestions.length > 0 && (
          <div class="hp-w-api-suggest">
            <small>In the answer — click to add:</small>
            {suggestions.slice(0, 16).map((p) => (
              <button
                type="button"
                class="hp-chip"
                title={String(p.sample)}
                onClick={() => set({ fields: [...draft.fields, { label: p.path.split(/[.\]]/).filter(Boolean).pop(), path: p.path, format: "auto" }] })}
              >
                {p.path || "(value)"}
              </button>
            ))}
          </div>
        )}
      </fieldset>

      <div class="hp-w-api-actions">
        <button type="submit" class="hp-button">
          Save
        </button>
        {options.url && (
          <button type="button" class="hp-button hp-button--quiet" onClick={onDone}>
            Cancel
          </button>
        )}
      </div>
    </form>
  );
}

export default function Api({ widget, options, setOptions, ctx }) {
  // Open the editor until the widget has an address and fields.
  const [editing, setEditing] = useState(!options.url || !options.fields?.length);
  const canFetch = ctx.capabilities?.has("widgets.proxy");
  const { data, error } = useWidgetData(widget, canFetch && !!options.url, JSON.stringify([options.url, options.headers]), widget.refresh ?? 60);

  if (!canFetch) return <p class="hp-widget-empty">API widgets need Haepage's Server mode, which keeps keys off the browser.</p>;

  const privateHost = error?.includes("private network") && (() => {
    try {
      return new URL(options.url.replace(/\{\{[^}]*\}\}/g, "x")).hostname;
    } catch {
      return null;
    }
  })();

  return (
    <div class="hp-w-api">
      {error && (
        <p class="hp-widget-empty" role="alert">
          {error}
          {privateHost && ctx.allowHost && (
            <>
              {" "}
              <button type="button" class="hp-link-button" onClick={() => ctx.allowHost(privateHost)}>
                Allow {privateHost}
              </button>
            </>
          )}
        </p>
      )}
      {editing ? (
        <Editor options={options} setOptions={setOptions} data={data} onDone={() => setEditing(false)} />
      ) : (
        <>
          {data !== undefined ? <View data={data} options={options} /> : !error && <p class="hp-widget-empty">Loading…</p>}
          <button type="button" class="hp-link-button hp-w-api-edit" onClick={() => setEditing(true)}>
            Edit
          </button>
        </>
      )}
    </div>
  );
}

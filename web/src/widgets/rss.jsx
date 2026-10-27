import { useState } from "preact/hooks";
import { safeUrl, useWidgetData } from "./api.js";
import { timeAgo } from "./lib.js";
import { NeedsServer, SetupForm } from "./setup.jsx";

// Latest entries of an RSS or Atom feed. options: url, limit (8).
export default function Rss({ widget, options, setOptions, ctx }) {
  const can = ctx.capabilities?.has("widgets.proxy");
  const [editing, setEditing] = useState(!options.url);
  const { data, error } = useWidgetData(widget, can && !!options.url, JSON.stringify([options.url, options.limit]), widget.refresh ?? 900);

  if (!can) return <NeedsServer />;
  if (editing) {
    return (
      <SetupForm
        label="Feed address"
        placeholder="https://example.com/feed.xml"
        help="RSS or Atom. A {{secrets.name}} in the address stays on the server."
        initial={options.url}
        onSave={(url) => {
          setOptions({ url });
          setEditing(false);
        }}
        onCancel={options.url && (() => setEditing(false))}
      />
    );
  }
  return (
    <div class="hp-w-feed">
      {error && <p class="hp-widget-empty" role="alert">{error}</p>}
      {data === undefined && !error && <p class="hp-widget-empty">Loading…</p>}
      {data?.items.length === 0 && <p class="hp-widget-empty">The feed is empty.</p>}
      {data?.items.length > 0 && (
        <ul>
          {data.items.map((it, i) => {
            const href = safeUrl(it.link);
            return (
              <li key={`${i}-${it.link}`} title={it.summary}>
                {href ? (
                  <a href={href} target="_blank" rel="noopener noreferrer">
                    {it.title || it.link}
                  </a>
                ) : (
                  <span>{it.title}</span>
                )}
                {it.date && <small>{timeAgo(it.date)}</small>}
              </li>
            );
          })}
        </ul>
      )}
      <button type="button" class="hp-link-button hp-w-api-edit" onClick={() => setEditing(true)}>
        Change feed{data?.title ? ` (${data.title})` : ""}
      </button>
    </div>
  );
}

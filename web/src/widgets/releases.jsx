import { useState } from "preact/hooks";
import { safeUrl, useWidgetData } from "./api.js";
import { timeAgo } from "./lib.js";
import { NeedsServer, SetupForm } from "./setup.jsx";

const NEW_FOR = 3 * 86400_000;

// Latest releases of the projects you run, from their public release feeds
// (no token needed). options: repos ["owner/name"], limit (6).
export default function Releases({ widget, options, setOptions, ctx }) {
  const can = ctx.capabilities?.has("widgets.proxy");
  const repos = options.repos ?? [];
  const [editing, setEditing] = useState(repos.length === 0);
  const { data, error } = useWidgetData(widget, can && repos.length > 0, JSON.stringify([repos, options.limit]), widget.refresh ?? 3600);

  if (!can) return <NeedsServer />;
  if (editing) {
    return (
      <SetupForm
        label="Projects on GitHub, one per line"
        placeholder={"owner/name\nowner/other-name"}
        multiline
        initial={repos.join("\n")}
        onSave={(text) => {
          setOptions({ repos: text.split(/\s*[\n,]\s*/).filter(Boolean) });
          setEditing(false);
        }}
        onCancel={repos.length > 0 && (() => setEditing(false))}
      />
    );
  }
  const now = Date.now();
  return (
    <div class="hp-w-releases">
      {error && <p class="hp-widget-empty" role="alert">{error}</p>}
      {data === undefined && !error && <p class="hp-widget-empty">Loading…</p>}
      {data?.items.length === 0 && data.errors.length === 0 && <p class="hp-widget-empty">No releases yet.</p>}
      {data?.items.length > 0 && (
        <ul>
          {data.items.map((r, i) => {
            const href = safeUrl(r.link);
            const fresh = r.date && now - Date.parse(r.date) < NEW_FOR;
            return (
              <li key={`${r.repo}-${i}`}>
                <span class="hp-w-releases-repo">{r.repo.split("/")[1]}</span>
                {href ? (
                  <a href={href} target="_blank" rel="noopener noreferrer">
                    {r.title}
                  </a>
                ) : (
                  <span>{r.title}</span>
                )}
                {fresh && <span class="hp-chip hp-chip--info">new</span>}
                <small>{timeAgo(r.date)}</small>
              </li>
            );
          })}
        </ul>
      )}
      {data?.errors.length > 0 && <p class="hp-widget-empty">Couldn't read: {data.errors.map((e) => e.repo).join(", ")}</p>}
      <button type="button" class="hp-link-button hp-w-api-edit" onClick={() => setEditing(true)}>
        Change projects
      </button>
    </div>
  );
}

import { useState } from "preact/hooks";
import { expiryRows, whenText } from "../expiry.js";
import { useWidgetData } from "./api.js";
import { SetupForm } from "./setup.jsx";

// When certificates and domains run out. Certificates come from the https
// services on the page (Server mode checks read them); domains are looked
// up by the server. options: domains [], warnDays (30).
export default function Expiry({ widget, options, setOptions, ctx }) {
  const domains = options.domains ?? [];
  const warn = options.warnDays ?? 30;
  const can = ctx.capabilities?.has("widgets.proxy");
  const [editing, setEditing] = useState(false);
  const { data, error } = useWidgetData(widget, can && domains.length > 0, JSON.stringify(domains), widget.refresh ?? 3600);

  if (editing) {
    return (
      <SetupForm
        label="Domains, one per line"
        placeholder={"example.com\nmy-home.net"}
        help="Haepage asks each domain's registry (RDAP) when it expires, about once a day."
        multiline
        initial={domains.join("\n")}
        onSave={(text) => {
          setOptions({ domains: text.split(/\s*[\n,]\s*/).filter(Boolean) });
          setEditing(false);
        }}
        onCancel={() => setEditing(false)}
      />
    );
  }

  const rows = expiryRows(ctx.services, data?.domains ?? []);
  const now = Date.now();
  return (
    <div class="hp-w-expiry">
      {error && <p class="hp-widget-empty" role="alert">{error}</p>}
      {rows.length === 0 && (
        <p class="hp-widget-empty">
          {can ? "No https services checked yet, and no domains to watch." : "Certificates are read by Server mode's checks."}
        </p>
      )}
      {rows.length > 0 && (
        <ul>
          {rows.map((r) => {
            const soon = r.expires && Date.parse(r.expires) - now < warn * 86400_000;
            return (
              <li key={`${r.kind}-${r.name}`} class={soon ? "is-soon" : ""} title={r.registrar ? `Registrar: ${r.registrar}` : undefined}>
                <span class="hp-w-expiry-kind">{r.kind === "cert" ? "Cert" : "Domain"}</span>
                <span class="hp-w-expiry-name">{r.name}</span>
                <small>{r.expires ? whenText(r.expires, now) : r.error}</small>
              </li>
            );
          })}
        </ul>
      )}
      {can && (
        <button type="button" class="hp-link-button hp-w-api-edit" onClick={() => setEditing(true)}>
          {domains.length ? "Change domains" : "Watch domains too"}
        </button>
      )}
    </div>
  );
}

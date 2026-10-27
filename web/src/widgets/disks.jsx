import { formatBytes, percent } from "../system.js";

// Every disk the server watches, fullest bars turning amber then red.
export default function Disks({ ctx }) {
  if (!ctx.system?.enabled) return <p class="hp-widget-empty">Disks need Haepage's Server mode.</p>;
  const disks = ctx.system.stats?.disks;
  if (!disks) return <p class="hp-widget-empty">Loading…</p>;
  if (disks.length === 0) return <p class="hp-widget-empty">No disks found.</p>;
  return (
    <ul class="hp-w-system">
      {disks.map((d) => {
        const pct = percent(d.used, d.total);
        return (
          <li key={d.path}>
            <span class="hp-w-system-top">
              <span>{d.path}</span>
              <strong>{pct}%</strong>
            </span>
            <span class="hp-bar" style={{ "--avatar-hue": pct >= 90 ? 4 : pct >= 75 ? 32 : 211 }}>
              <span class="hp-bar-fill" style={{ "--pct": `${pct}%` }} />
            </span>
            <small>
              {formatBytes(d.total - d.used)} free of {formatBytes(d.total)}
            </small>
          </li>
        );
      })}
    </ul>
  );
}

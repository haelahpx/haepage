import { formatBytes, mainDisk, percent } from "../system.js";
import { METRIC_HUES } from "../ui/GroupIcon.jsx";

// The server's CPU, memory and storage, from the stats the page already
// polls (Server mode only).
export default function System({ ctx }) {
  if (!ctx.system?.enabled) return <p class="hp-widget-empty">Server stats need Haepage's Server mode.</p>;
  const s = ctx.system.stats;
  if (!s) return <p class="hp-widget-empty">Loading…</p>;
  const disk = mainDisk(s.disks);
  const rows = [
    ["CPU", Math.round(s.cpu.percent), `${s.cpu.cores} cores`, METRIC_HUES.cpu],
    ["Memory", percent(s.memory.used, s.memory.total), `${formatBytes(s.memory.used)} of ${formatBytes(s.memory.total)}`, METRIC_HUES.memory],
    disk && ["Storage", percent(disk.used, disk.total), `${formatBytes(disk.used)} of ${formatBytes(disk.total)}`, METRIC_HUES.disk],
  ].filter(Boolean);
  return (
    <ul class="hp-w-system">
      {rows.map(([label, pct, detail, hue]) => (
        <li key={label}>
          <span class="hp-w-system-top">
            <span>{label}</span>
            <strong>{pct}%</strong>
          </span>
          <span class="hp-bar" style={{ "--avatar-hue": hue }}>
            <span class="hp-bar-fill" style={{ "--pct": `${pct}%` }} />
          </span>
          <small>{detail}</small>
        </li>
      ))}
    </ul>
  );
}

// Every temperature sensor the server has, hottest reading per device.
export default function Temperature({ ctx }) {
  if (!ctx.system?.enabled) return <p class="hp-widget-empty">Temperatures need Haepage's Server mode.</p>;
  const temps = ctx.system.stats?.temps;
  if (!temps) return <p class="hp-widget-empty">Loading…</p>;
  if (temps.length === 0) return <p class="hp-widget-empty">This machine reports no temperature sensors.</p>;
  return (
    <ul class="hp-w-system">
      {temps.map((t) => {
        const limit = t.critC ?? 100;
        const pct = Math.min(100, (t.c / limit) * 100);
        const hot = pct >= 85;
        return (
          <li key={t.name} class={hot ? "is-hot" : ""}>
            <span class="hp-w-system-top">
              <span>{t.name}</span>
              <strong>{Math.round(t.c)}°C</strong>
            </span>
            <span class="hp-bar" style={{ "--avatar-hue": hot ? 4 : pct >= 70 ? 32 : 145 }}>
              <span class="hp-bar-fill" style={{ "--pct": `${pct}%` }} />
            </span>
            {t.critC && <small>limit {Math.round(t.critC)}°C</small>}
          </li>
        );
      })}
    </ul>
  );
}

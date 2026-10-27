import { greeting, headline, summarize } from "../status.js";
import { formatBytes, formatRate, formatUptime, mainDisk, percent } from "../system.js";
import { Group } from "./Card.jsx";
import { IconTile, LineIcon, METRIC_HUES } from "./GroupIcon.jsx";
import { ClockTime, SettingsButton, formatDate, useNow } from "./Header.jsx";

function Hero({ title, onSettings, children }) {
  const now = useNow();
  return (
    <section class="hp-tile hp-tile--hero" aria-label="Overview">
      <div class="hp-hero-clock">
        <ClockTime now={now} />
      </div>
      <div class="hp-hero-main">
        <div class="hp-hero-eyebrow">
          <LineIcon name="home" size={18} class="hp-hero-home" />
          <h1 class="hp-title">{title}</h1>
          <span class="hp-clock-date">{formatDate(now)}</span>
        </div>
        <p class="hp-greeting">{greeting(now.getHours())}</p>
        {children}
      </div>
      <div class="hp-hero-actions">
        <SettingsButton onClick={onSettings} />
      </div>
    </section>
  );
}

// Tile header in the same shape as a group header: icon, title, subtitle.
function TileHead({ icon, hue, title, sub }) {
  return (
    <h2 class="hp-group-title">
      <IconTile icon={icon} hue={hue} />
      <span class="hp-group-name">
        {title}
        {sub && <span class="hp-group-sub">{sub}</span>}
      </span>
    </h2>
  );
}

// One mark per checked service. Themes show either the segments (one
// capsule per service) or the bar (up, down and unchecked portions).
function Meter({ statuses, counts }) {
  return (
    <div
      class="hp-meter"
      role="img"
      aria-label={`${counts.up} up, ${counts.down} down, ${counts.unknown} not checked`}
    >
      <div class="hp-meter-segments">
        {statuses.map((status, i) => (
          <span key={i} class={`hp-seg hp-seg--${status}`} />
        ))}
      </div>
      <div class="hp-meter-bar">
        {counts.up > 0 && <span class="hp-meter-up" style={{ flexGrow: String(counts.up) }} />}
        {counts.down > 0 && <span class="hp-meter-down" style={{ flexGrow: String(counts.down) }} />}
        {counts.unknown > 0 && <span class="hp-meter-rest" style={{ flexGrow: String(counts.unknown) }} />}
      </div>
    </div>
  );
}

function StatusTile({ health, services }) {
  const counts = summarize(health);
  const checked = services.filter((s) => health[s.id]);
  const down = checked.filter((s) => health[s.id].status === "down").map((s) => s.name);
  const state = counts.down > 0 ? "down" : counts.up > 0 ? "up" : "idle";

  return (
    <section class="hp-tile hp-tile--status" data-state={state} aria-label="Service status">
      <TileHead
        icon="pulse"
        hue={state === "down" ? 4 : 145}
        title="Status"
        sub={`${services.length} ${services.length === 1 ? "service" : "services"}`}
      />
      <p class="hp-status-big">
        <strong>{counts.total ? counts.up : "–"}</strong>
        {counts.total > 0 && <span>/{counts.total} up</span>}
      </p>
      <Meter statuses={checked.map((s) => health[s.id].status)} counts={counts} />
      <p class="hp-status-headline">{headline(counts)}</p>
      {down.length > 0 && (
        <ul class="hp-down-list">
          {down.slice(0, 3).map((name) => (
            <li key={name}>{name}</li>
          ))}
          {down.length > 3 && <li>+{down.length - 3} more</li>}
        </ul>
      )}
    </section>
  );
}

function Metric({ icon, label, value, pct, detail }) {
  const hue = METRIC_HUES[icon];
  return (
    <div class="hp-metric">
      <IconTile icon={icon} hue={hue} />
      <div class="hp-metric-body">
        <div class="hp-metric-top">
          <span class="hp-metric-label">{label}</span>
          <span class="hp-metric-value">{value}</span>
        </div>
        {pct != null && (
          <div class="hp-bar" style={{ "--avatar-hue": hue }}>
            <span class="hp-bar-fill" style={{ width: `${pct}%` }} />
          </div>
        )}
        {detail && <span class="hp-metric-detail">{detail}</span>}
      </div>
    </div>
  );
}

function SystemTile({ stats }) {
  if (!stats) {
    return (
      <section class="hp-tile hp-tile--system" aria-label="Server" aria-busy="true">
        <TileHead icon="server" hue={METRIC_HUES.server} title="Server" sub="Loading…" />
      </section>
    );
  }
  const disk = mainDisk(stats.disks);
  const memPct = percent(stats.memory.used, stats.memory.total);
  const diskPct = disk ? percent(disk.used, disk.total) : null;
  const os = stats.os?.replace(/\s*\(.*\)$/, ""); // "Fedora Linux 43 (Workstation Edition)" → "Fedora Linux 43"
  const foot = [`Up ${formatUptime(stats.uptime)}`, stats.tempC != null && `${Math.round(stats.tempC)}°C`, os]
    .filter(Boolean)
    .join(" · ");

  return (
    <section class="hp-tile hp-tile--system" aria-label="Server">
      <TileHead icon="server" hue={METRIC_HUES.server} title="Server" sub={stats.hostname} />
      <div class="hp-metrics">
        <Metric
          icon="cpu"
          label="CPU"
          value={`${Math.round(stats.cpu.percent)}%`}
          pct={stats.cpu.percent}
          detail={`${stats.cpu.cores} cores · load ${stats.cpu.load1.toFixed(2)}`}
        />
        <Metric
          icon="memory"
          label="Memory"
          value={`${memPct}%`}
          pct={memPct}
          detail={`${formatBytes(stats.memory.used)} of ${formatBytes(stats.memory.total)}`}
        />
        {disk && (
          <Metric
            icon="disk"
            label="Storage"
            value={`${diskPct}%`}
            pct={diskPct}
            detail={`${formatBytes(disk.used)} of ${formatBytes(disk.total)}`}
          />
        )}
        <Metric
          icon="network"
          label="Network"
          value={`↓ ${formatRate(stats.network.rxRate)}`}
          detail={`↑ ${formatRate(stats.network.txRate)}`}
        />
      </div>
      <p class="hp-system-foot">{foot}</p>
    </section>
  );
}

// Bento layout: hero (clock, greeting, search), status, server stats when
// the server provides them, then one tile per group.
export function Bento({ title, sections, services, health, system, search, empty, actions, onSettings }) {
  return (
    <div class="hp-bento">
      <Hero title={title} onSettings={onSettings}>
        {search}
      </Hero>
      <StatusTile health={health} services={services} />
      {system.enabled && <SystemTile stats={system.stats} />}
      {sections.length > 0 ? (
        sections.map((g) => <Group key={g.id} {...g} health={health} actions={actions} tile />)
      ) : (
        <section class="hp-tile hp-tile--wide">{empty}</section>
      )}
    </div>
  );
}

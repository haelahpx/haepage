import { useState } from "preact/hooks";
import { hostOf } from "../links.js";
import { certNotice } from "../expiry.js";
import { barStates, formatUptime } from "../uptime.js";
import { GroupIcon } from "./GroupIcon.jsx";
import { Icon } from "./Icon.jsx";

const STATUS_TEXT = { up: "Up", down: "Down", unknown: "Not checked" };

// `uptime` ({uptime, bars}, Server mode) shows on hover: the description
// becomes the 24-hour uptime and a strip of half-hour bars appears.
export function Card({ service, health, chip, uptime }) {
  const { link } = service;
  const [hover, setHover] = useState(false);
  const showUptime = hover && uptime;
  const cert = certNotice(health?.certExpires);
  const desc = showUptime ? `${formatUptime(uptime.uptime)} up in 24 h` : service.description || (link && hostOf(link.url));
  const title = link
    ? [`${link.url} (${link.kind})`, ...link.others.map((o) => `${o.url} (${o.kind})`)].join("\n")
    : "No URL set";

  return (
    <a
      class="hp-card"
      href={link?.url}
      title={title}
      aria-disabled={link ? undefined : "true"}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onFocus={() => setHover(true)}
      onBlur={() => setHover(false)}
    >
      <Icon name={service.name} icon={service.icon} />
      <div class="hp-card-body">
        <p class="hp-card-title">{service.name}</p>
        {desc && <p class="hp-card-desc">{desc}</p>}
      </div>
      {service.isNew && <span class="hp-badge hp-badge--new">New</span>}
      {chip && (
        <span class={`hp-chip hp-chip--${chip.tone}`} title={chip.title}>
          {chip.text}
        </span>
      )}
      {cert && (
        <span class={`hp-chip hp-chip--${cert.tone}`} title={cert.title}>
          {cert.text}
        </span>
      )}
      {health && (
        <span
          class={`hp-status hp-status--${health.status}`}
          role="img"
          aria-label={STATUS_TEXT[health.status]}
          title={health.reason ?? STATUS_TEXT[health.status]}
        />
      )}
      {health && health.status !== "unknown" && (
        <span class={`hp-status-label hp-status-label--${health.status}`} aria-hidden="true">
          {STATUS_TEXT[health.status]}
        </span>
      )}
      {showUptime && (
        <span class="hp-uptime" aria-hidden="true">
          {barStates(uptime.bars).map((s, i) => (
            <span key={i} class={`hp-uptime-bar hp-uptime-bar--${s}`} />
          ))}
        </span>
      )}
      <svg class="hp-card-chevron" viewBox="0 0 8 14" width="8" height="14" aria-hidden="true">
        <path d="M1 1l6 6-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </a>
  );
}

// A newly detected service. Add and Hide sit on the card's right edge;
// they are siblings of the card's link (buttons can't go inside a link),
// laid over the space the card leaves for them.
function InboxItem({ service, health, chip, actions }) {
  return (
    <div class="hp-inbox-item">
      <Card service={{ ...service, isNew: false }} health={health} chip={chip} />
      <div class="hp-inbox-actions">
        <button type="button" class="hp-button hp-inbox-add" onClick={() => actions.accept(service)}>
          Add
        </button>
        <button
          type="button"
          class="hp-icon-button hp-inbox-hide"
          aria-label={`Hide ${service.name}`}
          title="Hide"
          onClick={() => actions.hide(service)}
        >
          <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
            <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
          </svg>
        </button>
      </div>
    </div>
  );
}

// A group of cards. As a bento tile it widens when it holds more than four
// services. The Inbox group gets Add and Hide buttons when actions exist.
export function Group({ id, name, icon, services, health, chips = {}, uptime = {}, inbox, actions, tile = false, widgets = null }) {
  let cls = inbox ? "hp-group hp-group--inbox" : "hp-group";
  if (tile) cls += services.length > 4 ? " hp-tile hp-tile--wide" : " hp-tile";
  return (
    <section class={cls} aria-label={name}>
      <h2 class="hp-group-title">
        <GroupIcon id={id} name={name} icon={icon} />
        <span class="hp-group-name">
          {name}
          <span class="hp-group-sub">
            {services.length} {services.length === 1 ? "service" : "services"}
          </span>
        </span>
      </h2>
      <div class="hp-grid">
        {services.map((s) =>
          inbox && actions ? (
            <InboxItem key={s.id} service={s} health={health[s.id]} chip={chips[s.id]} actions={actions} />
          ) : (
            <Card key={s.id} service={s} health={health[s.id]} chip={chips[s.id]} uptime={uptime[s.id]} />
          ),
        )}
      </div>
      {widgets}
    </section>
  );
}

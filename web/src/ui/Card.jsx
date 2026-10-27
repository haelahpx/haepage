import { hostOf } from "../links.js";
import { GroupIcon } from "./GroupIcon.jsx";
import { Icon } from "./Icon.jsx";

const STATUS_TEXT = { up: "Up", down: "Down", unknown: "Not checked" };

export function Card({ service, health }) {
  const { link } = service;
  const desc = service.description || (link && hostOf(link.url));
  const title = link
    ? [`${link.url} (${link.kind})`, ...link.others.map((o) => `${o.url} (${o.kind})`)].join("\n")
    : "No URL set";

  return (
    <a class="hp-card" href={link?.url} title={title} aria-disabled={link ? undefined : "true"}>
      <Icon name={service.name} icon={service.icon} />
      <div class="hp-card-body">
        <p class="hp-card-title">{service.name}</p>
        {desc && <p class="hp-card-desc">{desc}</p>}
      </div>
      {service.isNew && <span class="hp-badge hp-badge--new">New</span>}
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
      <svg class="hp-card-chevron" viewBox="0 0 8 14" width="8" height="14" aria-hidden="true">
        <path d="M1 1l6 6-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </a>
  );
}

// A newly detected service with Add and Hide buttons. The buttons sit
// beside the card, not inside its link.
function InboxItem({ service, health, actions }) {
  return (
    <div class="hp-inbox-item">
      <Card service={service} health={health} />
      <div class="hp-inbox-actions">
        <button type="button" class="hp-button" onClick={() => actions.accept(service)}>
          Add
        </button>
        <button type="button" class="hp-button hp-button--quiet" onClick={() => actions.hide(service)}>
          Hide
        </button>
      </div>
    </div>
  );
}

// A group of cards. As a bento tile it widens when it holds more than four
// services. The Inbox group gets Add and Hide buttons when actions exist.
export function Group({ id, name, icon, services, health, inbox, actions, tile = false }) {
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
            <InboxItem key={s.id} service={s} health={health[s.id]} actions={actions} />
          ) : (
            <Card key={s.id} service={s} health={health[s.id]} />
          ),
        )}
      </div>
    </section>
  );
}

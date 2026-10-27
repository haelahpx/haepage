import { useState } from "preact/hooks";
import {
  KIND_NAMES,
  downloads,
  formatClock,
  formatDay,
  nowPlaying,
  pendingRequests,
  posterFor,
  timeAgo,
  upcoming,
} from "../integration.js";
import { IconTile, METRIC_HUES } from "./GroupIcon.jsx";

export function TileHead({ icon, title, sub }) {
  return (
    <h2 class="hp-group-title">
      <IconTile icon={icon} hue={METRIC_HUES[icon]} />
      <span class="hp-group-name">
        {title}
        {sub && <span class="hp-group-sub">{sub}</span>}
      </span>
    </h2>
  );
}

function Poster({ src, label }) {
  const [broken, setBroken] = useState(false);
  if (!src || broken) {
    return (
      <span class="hp-media-poster hp-media-poster--empty" aria-hidden="true">
        {[...(label ?? "?")][0]}
      </span>
    );
  }
  return <img class="hp-media-poster" src={src} alt="" loading="lazy" onError={() => setBroken(true)} />;
}

function Progress({ pct }) {
  return (
    <div class="hp-bar" style={{ "--avatar-hue": 280 }}>
      <span class="hp-bar-fill" style={{ "--pct": `${Math.max(0, Math.min(100, pct))}%` }} />
    </div>
  );
}

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

function NowPlaying({ sessions }) {
  return (
    <section class={sessions.length > 1 ? "hp-tile hp-tile--media hp-tile--wide" : "hp-tile hp-tile--media"} aria-label="Now playing">
      <TileHead icon="play" title="Now playing" sub={plural(sessions.length, "person watching", "people watching")} />
      <ul class="hp-media-list">
        {sessions.map((s) => (
          <li key={s.service + s.id} class="hp-media">
            <Poster src={posterFor(s)} label={s.title} />
            <div class="hp-media-body">
              <p class="hp-media-title">{s.title}</p>
              {s.subtitle && <p class="hp-media-sub">{s.subtitle}</p>}
              {s.runtimeMs > 0 && <Progress pct={(s.positionMs / s.runtimeMs) * 100} />}
              <p class="hp-media-meta">
                {s.user} · {s.device || s.client}
                {s.runtimeMs > 0 && ` · ${formatClock(s.positionMs)} / ${formatClock(s.runtimeMs)}`}
              </p>
            </div>
            <span class={`hp-chip ${s.paused ? "" : "hp-chip--live"}`}>
              {s.paused ? "Paused" : s.transcoding ? "Transcoding" : "Direct"}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Requests({ requests }) {
  return (
    <section class="hp-tile hp-tile--media" aria-label="Requests">
      <TileHead icon="request" title="Requests" sub={`${plural(requests.length, "request", "requests")} waiting`} />
      <ul class="hp-media-list">
        {requests.slice(0, 5).map((r) => (
          <li key={r.service + r.id} class="hp-media">
            <Poster src={r.poster} label={r.title} />
            <div class="hp-media-body">
              <p class="hp-media-title">{r.title}</p>
              <p class="hp-media-meta">
                {r.requestedBy || "Someone"} · {timeAgo(r.createdAt)}
              </p>
            </div>
            <span class="hp-chip">{r.type === "tv" ? "Series" : "Movie"}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Downloads({ items }) {
  return (
    <section class="hp-tile hp-tile--media" aria-label="Downloads">
      <TileHead icon="download" title="Downloads" sub={plural(items.length, "in the queue", "in the queue")} />
      <ul class="hp-media-list">
        {items.slice(0, 5).map((d) => (
          <li key={d.service + d.id} class="hp-media hp-media--compact">
            <div class="hp-media-body">
              <p class="hp-media-title">{d.title}</p>
              {d.subtitle && <p class="hp-media-sub">{d.subtitle}</p>}
              <Progress pct={d.progress} />
              <p class="hp-media-meta">
                {Math.round(d.progress)}%{d.timeLeft && ` · ${d.timeLeft} left`}
              </p>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Upcoming({ items }) {
  return (
    <section class="hp-tile hp-tile--media" aria-label="Coming up">
      <TileHead icon="calendar" title="Coming up" sub="Next 7 days" />
      <ul class="hp-media-list">
        {items.map((u, i) => (
          <li key={u.service + i} class="hp-media hp-media--compact">
            <div class="hp-media-body">
              <p class="hp-media-title">{u.title}</p>
              {u.subtitle && <p class="hp-media-sub">{u.subtitle}</p>}
            </div>
            <span class="hp-chip">{formatDay(u.date)}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Activity({ items }) {
  return (
    <section class="hp-tile hp-tile--media" aria-label="Activity">
      <TileHead icon="bell" title="Activity" sub="From your apps" />
      <ul class="hp-activity">
        {items.slice(0, 6).map((a) => (
          <li key={a.id}>
            <span class="hp-activity-title">{a.title}</span>
            <span class="hp-activity-meta">
              {a.detail ? `${a.detail} · ` : ""}
              {timeAgo(a.at)}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function ConnectApps({ apps, onConnect, onDismiss }) {
  return (
    <section class="hp-tile hp-tile--media" aria-label="Connect your apps">
      <TileHead icon="link" title="Live data" sub="Connect your apps" />
      <p class="hp-connect-intro">See who's watching, new requests and downloads right here.</p>
      <ul class="hp-connect-list">
        {apps.map((a) => (
          <li key={a.service}>
            <span>
              <strong>{a.name}</strong>
              <small>{KIND_NAMES[a.kind]}</small>
            </span>
            <button type="button" class="hp-button" onClick={() => onConnect(a.service)}>
              Connect
            </button>
          </li>
        ))}
      </ul>
      <button type="button" class="hp-link hp-connect-dismiss" onClick={onDismiss}>
        Not now
      </button>
    </section>
  );
}

// Integration tiles, in the order they matter most. Each appears only when
// it has something to show.
export function MediaTiles({ states, activity, unconnected, onConnect, onDismissConnect }) {
  const sessions = nowPlaying(states);
  const requests = pendingRequests(states);
  const queue = downloads(states);
  const soon = upcoming(states);
  return (
    <>
      {sessions.length > 0 && <NowPlaying sessions={sessions} />}
      {requests.length > 0 && <Requests requests={requests} />}
      {queue.length > 0 && <Downloads items={queue} />}
      {soon.length > 0 && <Upcoming items={soon} />}
      {activity.length > 0 && <Activity items={activity} />}
      {unconnected.length > 0 && <ConnectApps apps={unconnected} onConnect={onConnect} onDismiss={onDismissConnect} />}
    </>
  );
}

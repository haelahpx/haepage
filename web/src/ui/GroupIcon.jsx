// Line icons for group headers. A group's `icon` can name one of these or
// be an image URL; otherwise one is picked from the group's name.
const PATHS = {
  play: "M8 5v14l11-7z",
  wrench: "M14.7 6.3a4 4 0 0 0-5.2 5.2l-5.3 5.3a1.5 1.5 0 0 0 2.1 2.1l5.3-5.3a4 4 0 0 0 5.2-5.2l-2.4 2.4-2.1-.4-.4-2.1z",
  wifi: "M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0M2 9a14.5 14.5 0 0 1 20 0M12 19.5h.01",
  shield: "M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6z",
  download: "M12 4v11m-5-5 5 5 5-5M5 20h14",
  chart: "M5 20V11M12 20V5M19 20v-8",
  cloud: "M7 18a4 4 0 0 1-.6-8A6 6 0 0 1 18 9a4.5 4.5 0 0 1-.5 9z",
  home: "M4 11l8-7 8 7v9h-5v-6H9v6H4z",
  server: "M4 5h16v6H4zM4 13h16v6H4zM8 8h.01M8 16h.01",
  grid: "M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z",
  cpu: "M7 5h10a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2zM10 10h4v4h-4zM9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3",
  memory: "M3 8h18v9H3zM7 8v9M11 8v9M15 8v9M5 20v-3M19 20v-3",
  disk: "M4 6c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3zM4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3",
  network: "M7 20V5m0 0L3 9m4-4 4 4M17 4v15m0 0 4-4m-4 4-4-4",
  pulse: "M3 12h4l2.5-6 4 12 2.5-6H21",
  inbox: "M3 13h5l1.5 3h5l1.5-3h5M5.5 5h13L21 13v6H3v-6z",
  calendar: "M4 6h16v14H4zM4 10h16M8 3v4M16 3v4",
  bell: "M6 16V11a6 6 0 0 1 12 0v5l2 2H4zM10 20a2 2 0 0 0 4 0",
  request: "M12 5v14M5 12h14",
  link: "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1",
};

// Fixed colors for system metrics, like the system settings app.
export const METRIC_HUES = { cpu: 211, memory: 145, disk: 28, network: 265, server: 222, play: 280, request: 330, download: 150, calendar: 205, bell: 32, link: 222, tailnet: 190 };

// A colored icon square, styled like the group icons.
export function IconTile({ icon, hue }) {
  return (
    <span class="hp-group-icon" style={{ "--avatar-hue": hue }} aria-hidden="true">
      <LineIcon name={icon} />
    </span>
  );
}

const GUESSES = [
  [/media|movie|film|video|tv|music|audio|stream|photo/, "play"],
  [/download|torrent|usenet|arr\b/, "download"],
  [/net|dns|router|vpn|proxy/, "wifi"],
  [/secur|auth|password|vault/, "shield"],
  [/monitor|stat|metric|log|dashboard/, "chart"],
  [/cloud|sync|file|storage|backup|nas/, "cloud"],
  [/home|automation|smart|iot/, "home"],
  [/server|infra|system|admin|dev/, "server"],
  [/tool|util|productiv|office|note/, "wrench"],
];

export function guessGroupIcon(text) {
  const t = text.toLowerCase();
  return GUESSES.find(([re]) => re.test(t))?.[1] ?? "grid";
}

export function LineIcon({ name, size = 20, class: cls }) {
  return (
    <svg class={cls} viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
      <path
        d={PATHS[name] ?? PATHS.grid}
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>
  );
}

function hue(text) {
  let h = 0;
  for (const ch of text) h = (h * 31 + ch.codePointAt(0)) % 360;
  return h;
}

export function GroupIcon({ id, name, icon }) {
  const style = { "--avatar-hue": hue(id || name) };
  if (icon && /^(https?:|data:|\/|\.\/)/.test(icon)) {
    return <img class="hp-group-icon" src={icon} alt="" loading="lazy" />;
  }
  const glyph = icon && PATHS[icon] ? icon : guessGroupIcon(`${id} ${name}`);
  return (
    <span class="hp-group-icon" style={style} aria-hidden="true">
      <LineIcon name={glyph} />
    </span>
  );
}

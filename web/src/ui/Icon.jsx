import { useState } from "preact/hooks";

const looksLikeUrl = (s) => /^(https?:|data:|\/|\.\/)/.test(s) || /\.(svg|png|webp|avif|jpe?g|ico)$/i.test(s);

function hue(text) {
  let h = 0;
  for (const ch of text) h = (h * 31 + ch.codePointAt(0)) % 360;
  return h;
}

// A service icon from a URL, falling back to a colored letter so a broken
// or missing icon never leaves an empty box.
export function Icon({ name, icon }) {
  const [broken, setBroken] = useState(false);
  if (icon && looksLikeUrl(icon) && !broken) {
    return <img class="hp-card-icon" src={icon} alt="" loading="lazy" decoding="async" onError={() => setBroken(true)} />;
  }
  const letter = [...(name ?? "").trim()][0]?.toUpperCase() ?? "?";
  return (
    <span class="hp-card-icon hp-avatar" style={{ "--avatar-hue": hue(name ?? "") }} aria-hidden="true">
      {letter}
    </span>
  );
}

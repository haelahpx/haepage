import { useState } from "preact/hooks";
import { useEvery } from "./api.js";
import { clockParts } from "./lib.js";

// A clock for any time zone: options.timezone ("Asia/Tokyo"; empty for this
// device) and options.label (defaults to the zone's city).
export default function Clock({ options }) {
  const [now, setNow] = useState(() => new Date());
  useEvery(() => setNow(new Date()), 15);
  let parts;
  try {
    parts = clockParts(now, options.timezone);
  } catch {
    return <p class="hp-widget-empty">Unknown time zone "{options.timezone}".</p>;
  }
  return (
    <div class="hp-w-clock">
      <span class="hp-w-clock-time">{parts.time}</span>
      <span class="hp-w-clock-meta">{[options.label || parts.city, parts.day].filter(Boolean).join(" · ")}</span>
    </div>
  );
}

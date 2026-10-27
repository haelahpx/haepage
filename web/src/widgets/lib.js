// Pure helpers for the built-in widgets, kept apart from the JSX so they
// can be tested with node --test.

// Time, day and city name for a clock in `timezone` ("Asia/Tokyo"), or in
// this device's zone when it is empty. Throws on an unknown zone.
export function clockParts(date, timezone, locale) {
  const zone = timezone || undefined;
  const time = new Intl.DateTimeFormat(locale, { hour: "numeric", minute: "2-digit", timeZone: zone }).format(date);
  const day = new Intl.DateTimeFormat(locale, { weekday: "short", day: "numeric", month: "short", timeZone: zone }).format(date);
  const city = timezone ? timezone.split("/").pop().replaceAll("_", " ") : "";
  return { time, day, city };
}

// Adds a task with trimmed text; blank text leaves the list as it is.
export function addItem(items, text, id = Date.now().toString(36)) {
  const t = text.trim();
  return t ? [...items, { id, text: t, done: false }] : items;
}


// Weather comes from Open-Meteo, which needs no API key and allows browser
// requests. Units: "metric" (°C, km/h) or "imperial" (°F, mph).
export function forecastUrl({ latitude, longitude, units }) {
  const q = new URLSearchParams({
    latitude: String(latitude),
    longitude: String(longitude),
    current: "temperature_2m,apparent_temperature,weather_code,wind_speed_10m,relative_humidity_2m,is_day",
    daily: "weather_code,temperature_2m_max,temperature_2m_min",
    timezone: "auto",
    forecast_days: "4",
  });
  if (units === "imperial") {
    q.set("temperature_unit", "fahrenheit");
    q.set("wind_speed_unit", "mph");
  }
  return `https://api.open-meteo.com/v1/forecast?${q}`;
}

export function geocodeUrl(name) {
  return `https://geocoding-api.open-meteo.com/v1/search?${new URLSearchParams({ name, count: "5", format: "json" })}`;
}

// WMO weather codes → a label and one of the widget's icons.
export function describeWeather(code) {
  if (code === 0) return { label: "Clear", icon: "sun" };
  if (code <= 2) return { label: code === 1 ? "Mostly clear" : "Partly cloudy", icon: "partly" };
  if (code === 3) return { label: "Cloudy", icon: "cloud" };
  if (code === 45 || code === 48) return { label: "Fog", icon: "fog" };
  if (code >= 51 && code <= 57) return { label: "Drizzle", icon: "rain" };
  if ((code >= 61 && code <= 67) || (code >= 80 && code <= 82)) return { label: code >= 80 ? "Showers" : "Rain", icon: "rain" };
  if ((code >= 71 && code <= 77) || code === 85 || code === 86) return { label: "Snow", icon: "snow" };
  if (code >= 95) return { label: "Thunderstorm", icon: "storm" };
  return { label: "—", icon: "cloud" };
}

// "Mon", "Tue" for the forecast days, in the location's own calendar.
export function dayName(isoDate, locale) {
  return new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: "UTC" }).format(new Date(`${isoDate}T12:00:00Z`));
}

// Reads a value by path: "a.b[0].c", "a.b.0.c" or "" for the whole value.
export function getPath(value, path) {
  if (!path) return value;
  let cur = value;
  for (const part of path.replace(/\[(\d+)\]/g, ".$1").split(".").filter(Boolean)) {
    if (cur == null || typeof cur !== "object") return undefined;
    cur = cur[part];
  }
  return cur;
}

const UNITS = ["B", "KB", "MB", "GB", "TB", "PB"];

// Formats a value for display. Formats: auto, number, percent (0–100, or
// 0–1 is scaled), bytes, duration (seconds), date (ISO or Unix seconds), text.
export function formatValue(v, format = "auto", locale) {
  if (v === undefined || v === null || v === "") return "—";
  const n = typeof v === "number" ? v : Number(v);
  switch (format) {
    case "number":
      return Number.isFinite(n) ? new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(n) : String(v);
    case "percent":
      return Number.isFinite(n) ? `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(Math.abs(n) <= 1 && n !== 0 ? n * 100 : n)}%` : String(v);
    case "bytes": {
      if (!Number.isFinite(n)) return String(v);
      let i = 0;
      let x = n;
      while (Math.abs(x) >= 1024 && i < UNITS.length - 1) {
        x /= 1024;
        i++;
      }
      return `${new Intl.NumberFormat(locale, { maximumFractionDigits: i ? 1 : 0 }).format(x)} ${UNITS[i]}`;
    }
    case "duration": {
      if (!Number.isFinite(n)) return String(v);
      const d = Math.floor(n / 86400);
      const h = Math.floor((n % 86400) / 3600);
      const m = Math.floor((n % 3600) / 60);
      return d ? `${d}d ${h}h` : h ? `${h}h ${m}m` : m ? `${m}m` : `${Math.floor(n)}s`;
    }
    case "date": {
      const t = typeof v === "number" ? new Date(v < 1e12 ? v * 1000 : v) : new Date(v);
      return Number.isNaN(t.getTime()) ? String(v) : new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(t);
    }
    case "text":
      return typeof v === "object" ? JSON.stringify(v) : String(v);
    default:
      if (typeof v === "number") return new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(v);
      if (typeof v === "boolean") return v ? "Yes" : "No";
      return typeof v === "object" ? JSON.stringify(v) : String(v);
  }
}

// Paths to the simple values in an answer, for suggesting fields. Arrays
// show their first item only ("items[0].name").
export function leafPaths(value, limit = 40) {
  const out = [];
  const walk = (v, path) => {
    if (out.length >= limit) return;
    if (Array.isArray(v)) {
      if (v.length) walk(v[0], `${path}[0]`);
    } else if (v && typeof v === "object") {
      for (const k of Object.keys(v)) walk(v[k], path ? `${path}.${k}` : k);
    } else {
      out.push({ path, sample: v });
    }
  };
  walk(value, "");
  return out;
}

// Paths to the arrays in an answer, for choosing a list's items.
export function arrayPaths(value, limit = 20) {
  const out = [];
  const walk = (v, path) => {
    if (out.length >= limit || !v || typeof v !== "object") return;
    if (Array.isArray(v)) {
      out.push(path);
      if (v.length) walk(v[0], `${path}[0]`);
      return;
    }
    for (const k of Object.keys(v)) walk(v[k], path ? `${path}.${k}` : k);
  };
  walk(value, "");
  return out;
}

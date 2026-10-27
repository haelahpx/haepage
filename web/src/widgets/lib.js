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

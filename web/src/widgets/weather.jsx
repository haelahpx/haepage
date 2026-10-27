import { useState } from "preact/hooks";
import { useEvery } from "./api.js";
import { dayName, describeWeather, forecastUrl, geocodeUrl } from "./lib.js";

const ICONS = {
  sun: "M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10Zm0-5v2m0 16v2M4.2 4.2l1.4 1.4m12.8 12.8 1.4 1.4M2 12h2m16 0h2M4.2 19.8l1.4-1.4M18.4 5.6l1.4-1.4",
  partly: "M8 4v1.5M3.8 5.8l1 1M2 10h1.5M12.2 5.8l-1 1M5.5 11.5a3 3 0 0 1 5.4-2.3M9 19h9a4 4 0 0 0 0-8 5 5 0 0 0-9.6 1.5A3.3 3.3 0 0 0 9 19Z",
  cloud: "M7 19h10a5 5 0 0 0 .6-10A6 6 0 0 0 6.2 10.5 4.3 4.3 0 0 0 7 19Z",
  fog: "M4 10h16M6 14h12M4 18h16M8 6h8",
  rain: "M7 15h10a4.5 4.5 0 0 0 .5-9A5.5 5.5 0 0 0 7 7.5 3.8 3.8 0 0 0 7 15Zm1 3-1 3m5-3-1 3m5-3-1 3",
  snow: "M7 14h10a4.5 4.5 0 0 0 .5-9A5.5 5.5 0 0 0 7 6.5 3.8 3.8 0 0 0 7 14Zm1 4h.01M12 18h.01M16 18h.01M10 21h.01M14 21h.01",
  storm: "M7 14h10a4.5 4.5 0 0 0 .5-9A5.5 5.5 0 0 0 7 6.5 3.8 3.8 0 0 0 7 14Zm6 1-3 4h4l-3 4",
};

function Icon({ name, size = 22 }) {
  return (
    <svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d={ICONS[name]} />
    </svg>
  );
}

// Picks a place by name search or from the browser's location.
function Setup({ setOptions }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState(null);
  const [error, setError] = useState(null);
  const pick = (latitude, longitude, name) => setOptions({ latitude, longitude, name });

  return (
    <div class="hp-w-weather-setup">
      <form
        class="hp-w-form"
        onSubmit={async (e) => {
          e.preventDefault();
          setError(null);
          try {
            const res = await fetch(geocodeUrl(query.trim()));
            setResults((await res.json()).results ?? []);
          } catch {
            setError("The place search isn't answering.");
          }
        }}
      >
        <input placeholder="City" aria-label="City" value={query} onInput={(e) => setQuery(e.currentTarget.value)} />
        <button type="submit" class="hp-button" disabled={!query.trim()}>
          Find
        </button>
      </form>
      {results?.length === 0 && <p class="hp-widget-empty">No places found.</p>}
      {results?.length > 0 && (
        <ul class="hp-w-places">
          {results.map((r) => (
            <li key={r.id}>
              <button type="button" class="hp-link-button" onClick={() => pick(r.latitude, r.longitude, r.name)}>
                {[r.name, r.admin1, r.country].filter(Boolean).join(", ")}
              </button>
            </li>
          ))}
        </ul>
      )}
      {"geolocation" in navigator && (
        <button
          type="button"
          class="hp-link-button"
          onClick={() =>
            navigator.geolocation.getCurrentPosition(
              (p) => pick(Math.round(p.coords.latitude * 100) / 100, Math.round(p.coords.longitude * 100) / 100, ""),
              () => setError("Location isn't available here."),
            )
          }
        >
          Use my location
        </button>
      )}
      {error && <p class="hp-widget-empty">{error}</p>}
    </div>
  );
}

// Current weather and a three-day forecast. options: latitude, longitude,
// name, units ("metric" or "imperial").
export default function Weather({ options, setOptions, widget }) {
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const ready = options.latitude != null && options.longitude != null;

  useEvery(
    async () => {
      if (!ready) return;
      try {
        const res = await fetch(forecastUrl(options));
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        setData(await res.json());
        setError(null);
      } catch {
        setError("The weather service isn't answering.");
      }
    },
    ready ? (widget.refresh ?? 900) : 0,
  );

  if (!ready) return <Setup setOptions={setOptions} />;
  if (!data) return <p class="hp-widget-empty">{error ?? "Loading…"}</p>;

  const unit = data.current_units?.temperature_2m ?? "°";
  const now = describeWeather(data.current.weather_code);
  const wind = data.current_units?.wind_speed_10m ?? "";
  return (
    <div class="hp-w-weather">
      <div class="hp-w-weather-now">
        <Icon name={now.icon} size={40} />
        <span class="hp-w-weather-temp">
          {Math.round(data.current.temperature_2m)}
          {unit}
        </span>
        <span class="hp-w-weather-meta">
          <strong>{now.label}</strong>
          <span>
            {options.name && `${options.name} · `}feels {Math.round(data.current.apparent_temperature)}
            {unit} · {Math.round(data.current.wind_speed_10m)} {wind}
          </span>
        </span>
      </div>
      <ol class="hp-w-forecast">
        {data.daily.time.slice(1).map((day, i) => {
          const d = describeWeather(data.daily.weather_code[i + 1]);
          return (
            <li key={day} title={d.label}>
              <span>{dayName(day)}</span>
              <Icon name={d.icon} size={20} />
              <span>
                {Math.round(data.daily.temperature_2m_max[i + 1])}° <small>{Math.round(data.daily.temperature_2m_min[i + 1])}°</small>
              </span>
            </li>
          );
        })}
      </ol>
      {error && <p class="hp-widget-empty">{error} Showing the last forecast.</p>}
      <button
        type="button"
        class="hp-link-button hp-w-weather-change"
        onClick={() => setOptions({ latitude: undefined, longitude: undefined, name: undefined })}
      >
        Change place
      </button>
    </div>
  );
}

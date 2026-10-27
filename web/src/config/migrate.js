import { CONFIG_VERSION, emptyConfig } from "./defaults.js";

const LISTS = ["groups", "services", "widgets", "engines", "bookmarks"];

// Built-in themes that were renamed.
const RENAMED_THEMES = { android: "tonal", apple: "glass" };

// Brings a parsed haepage.json up to the current version and fills in
// defaults. Unknown fields are kept, so saving never loses data written by
// another Haepage version.
export function migrate(raw) {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    throw new Error("haepage.json must contain a JSON object");
  }
  const version = raw.version ?? CONFIG_VERSION;
  if (typeof version !== "number" || version < 1) {
    throw new Error(`haepage.json has an invalid version: ${JSON.stringify(version)}`);
  }
  if (version > CONFIG_VERSION) {
    throw new Error(
      `haepage.json is version ${version}, but this Haepage only understands up to version ${CONFIG_VERSION}. Update Haepage.`,
    );
  }

  const base = emptyConfig();
  const config = {
    ...base,
    ...raw,
    version: CONFIG_VERSION,
    settings: { ...base.settings, ...objectOr(raw.settings) },
    layout: { ...base.layout, ...objectOr(raw.layout) },
    overrides: objectOr(raw.overrides),
  };
  for (const key of LISTS) {
    if (!Array.isArray(config[key])) config[key] = [];
  }
  config.settings.theme = RENAMED_THEMES[config.settings.theme] ?? config.settings.theme;
  return config;
}

function objectOr(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

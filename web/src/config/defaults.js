export const CONFIG_VERSION = 1;

export const DEFAULT_SETTINGS = {
  title: "Home",
  theme: "tonal",
  style: "bento",
  scheme: "auto",
  fx: "auto",
  search: "duckduckgo",
  customCss: "",
  inbox: "inbox",
};

export function emptyConfig() {
  return {
    version: CONFIG_VERSION,
    settings: { ...DEFAULT_SETTINGS },
    groups: [],
    services: [],
    overrides: {},
    layout: { type: "grid", columns: "auto", profiles: {} },
    widgets: [],
    engines: [],
    bookmarks: [],
  };
}

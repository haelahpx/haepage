import { useCallback, useEffect, useMemo, useRef, useState } from "preact/hooks";
import { emptyConfig } from "./config/defaults.js";
import { migrate } from "./config/migrate.js";
import { ConflictError, clearDraft, loadConfig, loadServerConfig, saveDraft, saveServerConfig } from "./config/store.js";
import { SERVICE_EVENTS, applyEvent, setOverride } from "./events.js";
import { probe, unprobeable } from "./health.js";
import { arrange } from "./layout.js";
import { resolveLink } from "./links.js";
import { detectMode } from "./mode.js";
import { allEngines, findEngine } from "./search/engines.js";
import { buildIndex } from "./search/index.js";
import { applyCustomCss, applyTheme } from "./theme/apply.js";
import { resolveFx, resolveScheme } from "./theme/fx.js";
import { Bento } from "./ui/Bento.jsx";
import { Group } from "./ui/Card.jsx";
import { Header } from "./ui/Header.jsx";
import { Search } from "./ui/Search.jsx";
import { Settings } from "./ui/Settings.jsx";

const HEALTH_INTERVAL = 60_000;
const root = document.documentElement;
const darkQuery = window.matchMedia("(prefers-color-scheme: dark)");
const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");

function downloadJson(name, data) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2) + "\n"], { type: "application/json" }));
  const a = Object.assign(document.createElement("a"), { href: url, download: name });
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// Applies scheme, effect tier, theme and custom CSS. `settings` is null
// until the config loads, so the cached theme isn't replaced by a default.
function useAppearance(settings) {
  const [prefersDark, setPrefersDark] = useState(darkQuery.matches);
  const [themeError, setThemeError] = useState(null);
  const { scheme, fx, customCss, theme, title, style } = settings ?? {};
  const ready = settings !== null;

  useEffect(() => {
    if (ready) root.dataset.style = style === "simple" ? "simple" : "bento";
  }, [ready, style]);

  useEffect(() => {
    const onChange = () => setPrefersDark(darkQuery.matches);
    darkQuery.addEventListener("change", onChange);
    return () => darkQuery.removeEventListener("change", onChange);
  }, []);

  useEffect(() => {
    if (ready) root.dataset.scheme = resolveScheme(scheme, prefersDark);
  }, [ready, scheme, prefersDark]);

  useEffect(() => {
    if (ready) root.dataset.fx = resolveFx(fx, navigator, motionQuery.matches);
  }, [ready, fx]);

  useEffect(() => {
    if (ready) applyCustomCss(customCss);
  }, [ready, customCss]);

  useEffect(() => {
    if (!ready) return undefined;
    let current = true;
    applyTheme(theme)
      .then(() => current && setThemeError(null))
      .catch((err) => current && setThemeError(err.message));
    return () => {
      current = false;
    };
  }, [ready, theme]);

  useEffect(() => {
    if (ready) document.title = title || "Haepage";
  }, [ready, title]);

  return themeError;
}

// Probes each service's chosen URL every minute while the tab is visible.
function useHealth(services, enabled) {
  const [health, setHealth] = useState({});

  useEffect(() => {
    if (!enabled) return undefined;
    const targets = services.filter((s) => s.link && s.check?.type !== "none");
    let timer;
    let cancelled = false;

    const run = async () => {
      if (document.hidden) return;
      const results = await Promise.all(
        targets.map(async (s) => {
          const reason = unprobeable(s.link.url, location.protocol);
          return [s.id, reason ? { status: "unknown", reason } : { status: await probe(s.link.url) }];
        }),
      );
      if (!cancelled) setHealth(Object.fromEntries(results));
    };
    const onVisible = () => !document.hidden && run();

    setHealth(Object.fromEntries(targets.map((s) => [s.id, { status: "unknown" }])));
    run();
    timer = setInterval(run, HEALTH_INTERVAL);
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      cancelled = true;
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [services, enabled]);

  return health;
}

const SYSTEM_INTERVAL = 5_000;

// Polls /api/system while the tab is visible, if the server offers it.
function useSystem(enabled) {
  const [stats, setStats] = useState(null);

  useEffect(() => {
    if (!enabled) return undefined;
    let cancelled = false;
    const run = async () => {
      if (document.hidden) return;
      try {
        const res = await fetch("./api/system", { cache: "no-store" });
        if (res.ok && !cancelled) setStats(await res.json());
      } catch {
        // Keep the last reading; the next poll may succeed.
      }
    };
    const onVisible = () => !document.hidden && run();
    run();
    const timer = setInterval(run, SYSTEM_INTERVAL);
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      cancelled = true;
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [enabled]);

  return { enabled, stats };
}

// Follows the server's event stream. EventSource reconnects on its own,
// and every connection starts with a full snapshot.
function useLiveServices(enabled, onConfig) {
  const [services, setServices] = useState(null);
  const onConfigRef = useRef(onConfig);
  onConfigRef.current = onConfig;

  useEffect(() => {
    if (!enabled) return undefined;
    const source = new EventSource("./api/events");
    for (const type of SERVICE_EVENTS) {
      source.addEventListener(type, (e) => {
        const data = JSON.parse(e.data);
        setServices((list) => applyEvent(list ?? [], type, data));
      });
    }
    source.addEventListener("config", (e) => onConfigRef.current(JSON.parse(e.data).etag));
    return () => source.close();
  }, [enabled]);

  return services;
}

const SAVE_DELAY = 400;

export function App() {
  const [mode, setMode] = useState(null);
  const [state, setState] = useState(null); // { config, source, file, error }
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [importError, setImportError] = useState(null);

  const [notice, setNotice] = useState(null);
  const etag = useRef(null);
  const saveTimer = useRef(null);

  useEffect(() => {
    detectMode().then(async (m) => {
      root.dataset.mode = m.mode;
      setMode(m);
      if (m.capabilities.has("config.write")) {
        try {
          const loaded = await loadServerConfig();
          etag.current = loaded.etag;
          setState(loaded);
          return;
        } catch (err) {
          setNotice(`Could not load the config from the server (${err.message}).`);
        }
      }
      setState(await loadConfig());
    });
  }, []);

  const serverConfig = state?.source === "server";
  const config = state?.config ?? emptyConfig();
  const themeError = useAppearance(state ? config.settings : null);

  const reloadFromServer = useCallback(async () => {
    const loaded = await loadServerConfig();
    etag.current = loaded.etag;
    setState(loaded);
  }, []);

  // Server mode saves to the server, a moment after the last change so
  // typing doesn't send a request per key. Static mode keeps a draft.
  const update = useCallback(
    (next) => {
      setState((s) => ({ ...s, config: next, source: serverConfig ? "server" : "draft" }));
      if (!serverConfig) {
        saveDraft(next);
        return;
      }
      clearTimeout(saveTimer.current);
      saveTimer.current = setTimeout(async () => {
        try {
          etag.current = await saveServerConfig(next, etag.current);
          setNotice(null);
        } catch (err) {
          if (err instanceof ConflictError) {
            await reloadFromServer();
            setNotice("The settings were changed somewhere else, so the latest version was loaded. Please make your change again.");
          } else {
            setNotice(`Could not save: ${err.message}`);
          }
        }
      }, SAVE_DELAY);
    },
    [serverConfig, reloadFromServer],
  );

  // Another tab, an override, or an edit on disk changed the file.
  const onConfigEvent = useCallback(
    (newTag) => {
      if (serverConfig && newTag !== etag.current) reloadFromServer().catch(() => {});
    },
    [serverConfig, reloadFromServer],
  );
  const live = useLiveServices(mode?.capabilities.has("events") ?? false, onConfigEvent);

  const discard = () => {
    clearDraft();
    setState((s) => ({ ...s, config: s.file ?? emptyConfig(), source: s.file ? "file" : "defaults" }));
  };

  const importFile = async (file) => {
    try {
      update(migrate(JSON.parse(await file.text())));
      setImportError(null);
    } catch (err) {
      setImportError(`Could not import ${file.name}: ${err.message}`);
    }
  };

  const closeSettings = useCallback(() => setSettingsOpen(false), []);

  const services = useMemo(
    () =>
      (live ?? config.services)
        .filter((s) => s.state !== "hidden")
        .map((s) => ({ ...s, link: resolveLink(s.urls, location.hostname) })),
    [live, config.services],
  );
  const sections = useMemo(() => arrange(config.groups, services), [config.groups, services]);
  const engines = useMemo(() => allEngines(config.engines), [config.engines]);
  const defaultEngine = findEngine(engines, config.settings.search);
  const entries = useMemo(
    () => buildIndex({ services, bookmarks: config.bookmarks, groups: config.groups }),
    [services, config.bookmarks, config.groups],
  );
  const serverHealth = mode?.capabilities.has("health.full") ?? false;
  const browserHealth = useHealth(services, mode !== null && !serverHealth);
  const health = useMemo(
    () =>
      serverHealth
        ? Object.fromEntries(services.filter((s) => s.urls).map((s) => [s.id, s.health ?? { status: "unknown" }]))
        : browserHealth,
    [serverHealth, services, browserHealth],
  );

  // Add or hide a newly detected service, keeping any rename it already has.
  const actions = useMemo(() => {
    if (!mode?.capabilities.has("services.detect")) return null;
    const save = (s, change) =>
      setOverride(s.id, { ...config.overrides?.[s.id], ...change }).catch((err) => setNotice(`Could not save: ${err.message}`));
    return {
      accept: (s) => save(s, { pinned: true, hidden: false, group: s.suggestedGroup || "apps" }),
      hide: (s) => save(s, { hidden: true, pinned: false }),
    };
  }, [mode, config.overrides]);
  const system = useSystem(mode?.capabilities.has("system") ?? false);

  if (!state) return <main class="hp-app" aria-busy="true" />;

  const openSettings = () => setSettingsOpen(true);
  const bento = config.settings.style !== "simple";
  const search = <Search entries={entries} engines={engines} defaultEngine={defaultEngine} />;
  const empty = (
    <div class="hp-empty">
      <p>No services yet.</p>
      <p>
        {mode?.capabilities.has("services.detect")
          ? "Haepage is looking for services on this server. Start a container, or add services to haepage.json, or import a config from "
          : "Add them to haepage.json next to this page, or import a config from "}
        <button type="button" class="hp-link" onClick={openSettings}>
          Settings
        </button>
        .
      </p>
    </div>
  );

  return (
    <main class="hp-app">
      {!bento && <Header title={config.settings.title} onSettings={openSettings} />}

      {state.error && (
        <p class="hp-banner hp-banner--error" role="alert">
          haepage.json could not be loaded ({state.error}). Showing {state.source === "draft" ? "your browser copy" : "an empty page"}.
        </p>
      )}
      {themeError && (
        <p class="hp-banner hp-banner--error" role="alert">
          {themeError}
        </p>
      )}
      {notice && (
        <p class="hp-banner hp-banner--error" role="alert">
          {notice}
        </p>
      )}
      {state.source === "draft" && (
        <div class="hp-banner">
          <span>You have changes saved only in this browser.</span>
          <span class="hp-banner-actions">
            <button type="button" class="hp-button" onClick={() => downloadJson("haepage.json", config)}>
              Download haepage.json
            </button>
            <button type="button" class="hp-button hp-button--quiet" onClick={discard}>
              Discard
            </button>
          </span>
        </div>
      )}

      {bento ? (
        <Bento
          title={config.settings.title}
          sections={sections}
          services={services}
          health={health}
          system={system}
          search={search}
          empty={empty}
          actions={actions}
          onSettings={openSettings}
        />
      ) : (
        <>
          {search}
          {sections.length > 0
            ? sections.map((g) => <Group key={g.id} {...g} health={health} actions={actions} />)
            : empty}
        </>
      )}

      {settingsOpen && (
        <Settings
          config={config}
          engines={engines}
          serverSaves={serverConfig}
          error={importError}
          onChange={update}
          onExport={() => downloadJson("haepage.json", config)}
          onImport={importFile}
          onClose={closeSettings}
        />
      )}
    </main>
  );
}

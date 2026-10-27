import { useCallback, useEffect, useMemo, useRef, useState } from "preact/hooks";
import { emptyConfig } from "./config/defaults.js";
import { migrate } from "./config/migrate.js";
import { ConflictError, clearDraft, loadConfig, loadServerConfig, saveDraft, saveServerConfig } from "./config/store.js";
import { LIVE_EVENTS, applyLive, setOverride } from "./events.js";
import { probe, unprobeable } from "./health.js";
import { chipFor, shouldNotify } from "./integration.js";
import { arrange } from "./layout.js";
import { resolveLink } from "./links.js";
import { detectMode } from "./mode.js";
import { allEngines, findEngine } from "./search/engines.js";
import { buildIndex } from "./search/index.js";
import { applyCustomCss, applyTheme, importTheme, isThemeUrl, themeFromLink } from "./theme/apply.js";
import { applyWallpaper, listWallpapers, wallpaperUrls } from "./theme/wallpaper.js";
import { WidgetTile } from "./widgets/host.jsx";
import { placeWidgets } from "./widgets/registry.js";
import { hardLite, measureFrames, readMeasuredFx, resolveFx, resolveScheme, saveMeasuredFx, tierFromFrames } from "./theme/fx.js";
import { Bento } from "./ui/Bento.jsx";
import { Group } from "./ui/Card.jsx";
import { ConnectDialog } from "./ui/Connect.jsx";
import { Header } from "./ui/Header.jsx";
import { MediaTiles } from "./ui/Media.jsx";
import { Search } from "./ui/Search.jsx";
import { Settings } from "./ui/Settings.jsx";
import { Toasts, useToasts } from "./ui/Toasts.jsx";
import { browserStorage } from "./config/store.js";

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
// `serverThemes` loads link themes through the server's pinned copy.
function useAppearance(settings, serverThemes) {
  const [prefersDark, setPrefersDark] = useState(darkQuery.matches);
  const [themeError, setThemeError] = useState(null);
  const [themeVersion, setThemeVersion] = useState(0);
  const [measuredFx, setMeasuredFx] = useState(() => readMeasuredFx(browserStorage()));
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
    if (ready) root.dataset.fx = resolveFx(fx, navigator, motionQuery.matches, measuredFx);
  }, [ready, fx, measuredFx]);

  // Measures this device once, after the first paint, while fx is "auto".
  const measuring = ready && fx === "auto" && measuredFx === null && !hardLite(navigator, motionQuery.matches);
  useEffect(() => {
    if (!measuring) return undefined;
    let current = true;
    measureFrames().then((intervals) => {
      const tier = tierFromFrames(intervals);
      if (!current || !tier) return;
      saveMeasuredFx(browserStorage(), tier);
      setMeasuredFx(tier);
    });
    return () => {
      current = false;
    };
  }, [measuring]);

  const remeasureFx = useCallback(() => {
    saveMeasuredFx(browserStorage(), null);
    setMeasuredFx(null);
  }, []);

  useEffect(() => {
    if (ready) applyCustomCss(customCss);
  }, [ready, customCss]);

  useEffect(() => {
    if (!ready) return undefined;
    let current = true;
    applyTheme(theme, { server: serverThemes, force: themeVersion > 0 })
      .then(() => current && setThemeError(null))
      .catch((err) => current && setThemeError(err.message));
    return () => {
      current = false;
    };
  }, [ready, theme, serverThemes, themeVersion]);

  // Fetches the link theme again (it is pinned until then) and reapplies it.
  const updateTheme = useCallback(async () => {
    if (serverThemes) await importTheme(theme, { refresh: true });
    setThemeVersion((v) => v + 1);
  }, [serverThemes, theme]);

  useEffect(() => {
    if (ready) document.title = title || "Haepage";
  }, [ready, title]);

  const detectedFx = ready ? resolveFx("auto", navigator, motionQuery.matches, measuredFx) : null;
  return { themeError, detectedFx: measuring ? null : detectedFx, remeasureFx, updateTheme };
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
const TAILSCALE_INTERVAL = 30_000;
const HISTORY_INTERVAL = 300_000;

// Polls a JSON endpoint while the tab is visible. Keeps the last good
// reading when a poll fails; null until the first one arrives.
function usePolled(url, enabled, interval) {
  const [data, setData] = useState(null);

  useEffect(() => {
    if (!enabled) return undefined;
    let cancelled = false;
    const run = async () => {
      if (document.hidden) return;
      try {
        const res = await fetch(url, { cache: "no-store" });
        if (res.ok && !cancelled) setData(await res.json());
      } catch {
        // The next poll may succeed.
      }
    };
    const onVisible = () => !document.hidden && run();
    run();
    const timer = setInterval(run, interval);
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      cancelled = true;
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [url, enabled, interval]);

  return data;
}

// Follows the server's event stream: services, integration data and
// activity. EventSource reconnects on its own, and every connection starts
// with a full snapshot. onActivity sees only new activity, not the snapshot.
function useLive(enabled, onConfig, onActivity, onOpen) {
  const [live, setLive] = useState(null);
  const handlers = useRef({ onConfig, onActivity, onOpen });
  handlers.current = { onConfig, onActivity, onOpen };

  useEffect(() => {
    if (!enabled) return undefined;
    // The stream's snapshot replaces this; offline, the service worker's
    // cached copy is all there is.
    fetch("./api/services")
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((services) => setLive((state) => state ?? applyLive(null, "snapshot", { services })))
      .catch(() => {});
    const source = new EventSource("./api/events");
    for (const type of LIVE_EVENTS) {
      source.addEventListener(type, (e) => {
        const data = JSON.parse(e.data);
        setLive((state) => applyLive(state ?? applyLive(null, "snapshot", {}), type, data));
        if (type === "activity") handlers.current.onActivity(data);
      });
    }
    source.addEventListener("config", (e) => handlers.current.onConfig(JSON.parse(e.data).etag));
    source.addEventListener("open", () => handlers.current.onOpen?.());
    return () => source.close();
  }, [enabled]);

  return live;
}

const DISMISS_KEY = "haepage:connect-dismissed";

function readDismissed() {
  try {
    return JSON.parse(browserStorage()?.getItem(DISMISS_KEY) ?? "[]");
  } catch {
    return [];
  }
}

// Services with an integration, and whether each is connected.
function useApps(enabled, signature) {
  const [apps, setApps] = useState([]);
  const refresh = useCallback(async () => {
    if (!enabled) return;
    try {
      const res = await fetch("./api/integrations", { cache: "no-store" });
      if (res.ok) setApps(await res.json());
    } catch {
      // Try again on the next change.
    }
  }, [enabled]);
  useEffect(() => {
    refresh();
  }, [refresh, signature]);
  return [apps, refresh];
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
  const serverThemes = mode?.capabilities.has("themes.import") ?? false;
  const { themeError, detectedFx, remeasureFx, updateTheme } = useAppearance(state ? config.settings : null, serverThemes);

  // A shared link (?theme=https://…) offers a theme; nothing changes until
  // the visitor accepts.
  const [offeredTheme, setOfferedTheme] = useState(() => themeFromLink(location.search));
  const dismissOffer = useCallback(() => {
    setOfferedTheme(null);
    const url = new URL(location.href);
    url.searchParams.delete("theme");
    history.replaceState(history.state, "", url);
  }, []);
  const offer = offeredTheme && state && offeredTheme !== config.settings.theme ? offeredTheme : null;

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
  const { toasts, push: pushToast, dismiss, hold, release } = useToasts();
  const settingsRef = useRef(config.settings);
  settingsRef.current = config.settings;
  const onActivity = useCallback(
    (a) => {
      if (!shouldNotify(a)) return;
      pushToast({ id: a.id, title: a.title, detail: a.detail, image: a.image });
      if (document.hidden && settingsRef.current.desktopNotify !== false && globalThis.Notification?.permission === "granted") {
        try {
          new Notification(a.title, { body: a.detail ?? "", icon: a.image || undefined, tag: a.id });
        } catch {
          // Some browsers only allow notifications from a service worker.
        }
      }
    },
    [pushToast],
  );
  // The stream connected: if the page loaded from the offline cache, the
  // server is back, so drop the notice and load the current config.
  const onLiveOpen = useCallback(() => {
    if (!mode?.offline) return;
    setMode((m) => ({ ...m, offline: false }));
    reloadFromServer().catch(() => {});
  }, [mode?.offline, reloadFromServer]);
  const live = useLive(mode?.capabilities.has("events") ?? false, onConfigEvent, onActivity, onLiveOpen);
  const integrationStates = live?.integrations ?? {};
  const activity = live?.activity ?? [];
  const appSignature = (live?.services ?? [])
    .filter((s) => s.integration)
    .map((s) => s.id)
    .concat(Object.keys(integrationStates))
    .join(",");
  const [apps, refreshApps] = useApps(mode?.capabilities.has("integrations") ?? false, appSignature);
  const [connectTarget, setConnectTarget] = useState(null);
  const [dismissed, setDismissed] = useState(readDismissed);
  const dismissConnect = () => {
    const ids = apps.map((a) => a.service);
    setDismissed(ids);
    try {
      browserStorage()?.setItem(DISMISS_KEY, JSON.stringify(ids));
    } catch {
      // Only this visit, then.
    }
  };

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
      (live?.services ?? config.services)
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
        ? Object.fromEntries(services.filter((s) => s.urls && !s.noCheck).map((s) => [s.id, s.health ?? { status: "unknown" }]))
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
  const systemEnabled = mode?.capabilities.has("system") ?? false;
  const system = { enabled: systemEnabled, stats: usePolled("./api/system", systemEnabled, SYSTEM_INTERVAL) };
  const historyEnabled = mode?.capabilities.has("history") ?? false;
  const uptime = usePolled("./api/history", historyEnabled, HISTORY_INTERVAL)?.services ?? {};
  const tailscaleEnabled = mode?.capabilities.has("tailscale.devices") ?? false;
  const tailnet = { enabled: tailscaleEnabled, data: usePolled("./api/tailscale/devices", tailscaleEnabled, TAILSCALE_INTERVAL) };

  // Widgets: saved back into haepage.json when they change (notes, to-dos,
  // a weather place…), like any other setting.
  const updateWidget = useCallback(
    (next) => update({ ...config, widgets: config.widgets.map((w) => (w.id === next.id ? next : w)) }),
    [config, update],
  );
  const widgetCtx = {
    system,
    services,
    capabilities: mode?.capabilities ?? new Set(),
    // Lets an API widget reach a host on the home network (settings.proxyAllow).
    allowHost: (host) =>
      update({ ...config, settings: { ...config.settings, proxyAllow: [...new Set([...(config.settings.proxyAllow ?? []), host])] } }),
  };
  const placed = placeWidgets(config.widgets, sections.map((g) => g.id));
  const renderWidget = (w, inline) => <WidgetTile key={w.id} widget={w} ctx={widgetCtx} onChange={updateWidget} inline={inline} />;
  const widgetSlots = {
    top: placed.top.map((w) => renderWidget(w, false)),
    bottom: placed.bottom.map((w) => renderWidget(w, false)),
    inGroup: (id) => placed.groups[id]?.map((w) => renderWidget(w, true)) ?? null,
  };
  const chips = useMemo(
    () => Object.fromEntries(Object.entries(integrationStates).map(([id, st]) => [id, chipFor(st)]).filter(([, c]) => c)),
    [integrationStates],
  );
  const unconnected = apps.filter((a) => !a.connected && !dismissed.includes(a.service));
  const media = (
    <MediaTiles
      states={integrationStates}
      activity={activity}
      unconnected={unconnected}
      onConnect={setConnectTarget}
      onDismissConnect={dismissConnect}
    />
  );
  const connectApp = apps.find((a) => a.service === connectTarget);

  if (!state) return <main class="hp-app" aria-busy="true" />;

  // Server mode lists the wallpaper folder; null until it has loaded, so a
  // folder name isn't mistaken for a path next to index.html.
  const canWallpapers = mode?.capabilities.has("wallpapers") ?? false;
  const [wallpapers, setWallpapers] = useState(null);
  const refreshWallpapers = useCallback(() => listWallpapers().then(setWallpapers, () => setWallpapers([])), []);
  useEffect(() => {
    if (canWallpapers) refreshWallpapers();
  }, [canWallpapers, refreshWallpapers]);
  const wallpaperSetting = state ? config.settings.wallpaper : undefined;
  useEffect(() => {
    if (!state || (canWallpapers && wallpapers === null)) return;
    applyWallpaper(wallpaperUrls(wallpaperSetting, wallpapers ?? [], { screenWidth: screen.width, dpr: devicePixelRatio }));
  }, [state !== null, wallpaperSetting, wallpapers, canWallpapers]);

  const openSettings = () => {
    if (canWallpapers) refreshWallpapers();
    setSettingsOpen(true);
  };
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
      {mode?.offline && (
        <p class="hp-banner" role="status">
          Can't reach the Haepage server. Showing what it last sent; changes can't be saved until it's back.
        </p>
      )}
      {notice && (
        <p class="hp-banner hp-banner--error" role="alert">
          {notice}
        </p>
      )}
      {offer && (
        <div class="hp-banner" role="dialog" aria-label="Theme from a link">
          <span>
            This link offers a theme from <strong>{new URL(offer).host}</strong>.{" "}
            {serverConfig ? "Using it changes the page for everyone." : "Using it changes the page in this browser."}
          </span>
          <span class="hp-banner-actions">
            <button
              type="button"
              class="hp-button"
              onClick={() => {
                update({ ...config, settings: { ...config.settings, theme: offer } });
                dismissOffer();
              }}
            >
              Use theme
            </button>
            <button type="button" class="hp-button hp-button--quiet" onClick={dismissOffer}>
              Not now
            </button>
          </span>
        </div>
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
          chips={chips}
          uptime={uptime}
          system={system}
          tailnet={tailnet}
          media={media}
          widgets={widgetSlots}
          search={search}
          empty={empty}
          actions={actions}
          onSettings={openSettings}
        />
      ) : (
        <>
          {search}
          <div class="hp-integrations">{media}</div>
          {widgetSlots.top.length > 0 && <div class="hp-widgets">{widgetSlots.top}</div>}
          {sections.length > 0
            ? sections.map((g) => (
                <Group key={g.id} {...g} health={health} chips={chips} uptime={uptime} actions={actions} widgets={widgetSlots.inGroup(g.id)} />
              ))
            : empty}
          {widgetSlots.bottom.length > 0 && <div class="hp-widgets">{widgetSlots.bottom}</div>}
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
          detectedFx={detectedFx}
          onUpdateTheme={isThemeUrl(config.settings.theme) ? updateTheme : null}
          serverThemes={serverThemes}
          canLint={mode?.capabilities.has("themes.lint") ?? false}
          wallpapers={canWallpapers ? (wallpapers ?? []) : null}
          groups={sections.filter((g) => !g.inbox).map((g) => ({ id: g.id, name: g.name }))}
          canSecrets={mode?.capabilities.has("secrets") ?? false}
          onRemeasureFx={remeasureFx}
          apps={apps}
          onManageApp={(id) => {
            setSettingsOpen(false);
            setConnectTarget(id);
          }}
          onClose={closeSettings}
        />
      )}
      {connectApp && <ConnectDialog app={connectApp} onClose={() => setConnectTarget(null)} onChanged={refreshApps} />}
      <Toasts toasts={toasts} dismiss={dismiss} hold={hold} release={release} />
    </main>
  );
}

import { useEffect, useRef, useState } from "preact/hooks";
import { KIND_NAMES } from "../integration.js";
import { BUILTIN_THEMES, isThemeUrl, lintCss, shareLink } from "../theme/apply.js";
import { WIDGET_TYPES, newWidget } from "../widgets/registry.js";

const FX_NAMES = { full: "Full", balanced: "Balanced", lite: "Lite" };
const SHOWN_FINDINGS = 5;

// Lints `target` ({url} or {css}) on the server a moment after it stops
// changing. A link theme may still be importing, so a 404 is retried.
function useLint(enabled, target) {
  const [findings, setFindings] = useState([]);
  const key = enabled && target ? JSON.stringify(target) : "";
  useEffect(() => {
    if (!key) {
      setFindings([]);
      return undefined;
    }
    let cancelled = false;
    let timer;
    const run = (tries) =>
      lintCss(JSON.parse(key)).then(
        (f) => !cancelled && setFindings(f),
        () => {
          if (!cancelled && tries > 0) timer = setTimeout(() => run(tries - 1), 1500);
        },
      );
    timer = setTimeout(() => run(3), 500);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [key]);
  return findings;
}

function ThemeChecks({ findings }) {
  if (findings.length === 0) return null;
  const errors = findings.some((f) => f.severity === "error");
  return (
    <div class={`hp-checks${errors ? " hp-checks--error" : ""}`} role="status">
      <strong>
        {findings.length === 1 ? "1 thing" : `${findings.length} things`} may slow this theme down or stop it working
      </strong>
      <ul>
        {findings.slice(0, SHOWN_FINDINGS).map((f) => (
          <li key={`${f.line}-${f.rule}-${f.message}`}>
            {f.line > 0 && <span class="hp-checks-line">Line {f.line}</span>} {f.message}
          </li>
        ))}
      </ul>
      {findings.length > SHOWN_FINDINGS && <small>and {findings.length - SHOWN_FINDINGS} more</small>}
    </div>
  );
}

function Field({ label, children }) {
  return (
    <label class="hp-field">
      <span>{label}</span>
      {children}
    </label>
  );
}

export function Settings({
  config,
  engines,
  serverSaves,
  error,
  detectedFx,
  onRemeasureFx,
  onUpdateTheme,
  serverThemes,
  canLint,
  wallpapers,
  groups = [],
  apps = [],
  onManageApp,
  onChange,
  onExport,
  onImport,
  onClose,
}) {
  const s = config.settings;
  const set = (patch) => onChange({ ...config, settings: { ...s, ...patch } });
  const customTheme = isThemeUrl(s.theme);
  const [showUrl, setShowUrl] = useState(customTheme);
  const panel = useRef(null);
  const [permission, setPermission] = useState(globalThis.Notification?.permission ?? "unsupported");
  const [themeNote, setThemeNote] = useState(null);
  const wallpaperInFolder = !!wallpapers?.some((w) => w.name === s.wallpaper);
  const [wallpaperLink, setWallpaperLink] = useState(!!s.wallpaper && !wallpaperInFolder);
  const themeFindings = useLint(canLint && serverThemes && customTheme, { url: s.theme });
  const cssFindings = useLint(canLint && !!s.customCss?.trim(), { css: s.customCss });
  const flashNote = (text) => {
    setThemeNote(text);
    setTimeout(() => setThemeNote(null), 2500);
  };

  useEffect(() => {
    panel.current?.focus();
    const onKey = (e) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div class="hp-drawer-backdrop" onClick={onClose}>
      <aside
        ref={panel}
        class="hp-drawer"
        role="dialog"
        aria-modal="true"
        aria-label="Settings"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
      >
        <div class="hp-drawer-head">
          <h2>Settings</h2>
          <button type="button" class="hp-icon-button" onClick={onClose} aria-label="Close settings">
            ✕
          </button>
        </div>

        <Field label="Title">
          <input value={s.title} onInput={(e) => set({ title: e.currentTarget.value })} />
        </Field>

        <Field label="Layout">
          <select value={s.style} onChange={(e) => set({ style: e.currentTarget.value })}>
            <option value="bento">Bento</option>
            <option value="simple">Simple</option>
          </select>
        </Field>

        <Field label="Theme">
          <select
            value={showUrl ? "custom" : s.theme}
            onChange={(e) => {
              const v = e.currentTarget.value;
              setShowUrl(v === "custom");
              if (v !== "custom") set({ theme: v });
            }}
          >
            {BUILTIN_THEMES.map((t) => (
              <option value={t.id}>{t.name}</option>
            ))}
            <option value="custom">From a link…</option>
          </select>
        </Field>
        {showUrl && (
          <Field label="Theme link (https://…/style.css)">
            <input
              type="url"
              placeholder="https://example.com/theme.css"
              value={customTheme ? s.theme : ""}
              onChange={(e) => isThemeUrl(e.currentTarget.value) && set({ theme: e.currentTarget.value })}
            />
          </Field>
        )}
        {showUrl && customTheme && (
          <p class="hp-field-note">
            {themeNote ??
              (serverThemes
                ? "The server keeps a copy, so the theme only changes when you update it."
                : "Loaded by this browser from the link.")}{" "}
            {onUpdateTheme && (
              <button
                type="button"
                class="hp-link-button"
                onClick={() =>
                  onUpdateTheme().then(
                    () => flashNote("Updated."),
                    (err) => flashNote(err.message),
                  )
                }
              >
                Update
              </button>
            )}{" "}
            ·{" "}
            <button
              type="button"
              class="hp-link-button"
              onClick={() =>
                navigator.clipboard?.writeText(shareLink(s.theme)).then(
                  () => flashNote("Share link copied."),
                  () => flashNote(shareLink(s.theme)),
                )
              }
            >
              Copy share link
            </button>
          </p>
        )}
        {showUrl && customTheme && <ThemeChecks findings={themeFindings} />}

        <Field label="Wallpaper">
          <select
            value={wallpaperLink ? "link" : (s.wallpaper ?? "")}
            onChange={(e) => {
              const v = e.currentTarget.value;
              setWallpaperLink(v === "link");
              if (v !== "link") set({ wallpaper: v || undefined });
            }}
          >
            <option value="">None</option>
            {wallpapers?.map((w) => (
              <option value={w.name}>{w.name}</option>
            ))}
            <option value="link">From a link or path…</option>
          </select>
        </Field>
        {wallpaperLink && (
          <Field label="Image link or path">
            <input
              placeholder="https://example.com/sky.jpg or backgrounds/sky.jpg"
              value={wallpaperInFolder ? "" : (s.wallpaper ?? "")}
              onChange={(e) => set({ wallpaper: e.currentTarget.value.trim() || undefined })}
            />
          </Field>
        )}
        {wallpapers && !wallpaperLink && (
          <p class="hp-field-note">
            {wallpapers.length
              ? "Add more by putting images in the wallpapers folder in Haepage's data folder."
              : "Put JPEG, PNG or WebP images in the wallpapers folder in Haepage's data folder and they show up here."}
          </p>
        )}

        <Field label="Color scheme">
          <select value={s.scheme} onChange={(e) => set({ scheme: e.currentTarget.value })}>
            <option value="auto">Match the device</option>
            <option value="light">Light</option>
            <option value="dark">Dark</option>
          </select>
        </Field>

        <Field label="Effects">
          <select value={s.fx} onChange={(e) => set({ fx: e.currentTarget.value })}>
            <option value="auto">Automatic</option>
            <option value="full">Full</option>
            <option value="balanced">Balanced</option>
            <option value="lite">Lite (fastest)</option>
          </select>
        </Field>
        {s.fx === "auto" && (
          <p class="hp-field-note">
            {detectedFx ? `This device uses ${FX_NAMES[detectedFx]}.` : "Checking this device…"}{" "}
            {detectedFx && (
              <button type="button" class="hp-link-button" onClick={onRemeasureFx}>
                Check again
              </button>
            )}
          </p>
        )}

        <Field label="Web search">
          <select value={s.search} onChange={(e) => set({ search: e.currentTarget.value })}>
            {engines.map((e) => (
              <option value={e.id}>
                {e.name} (!{e.prefix})
              </option>
            ))}
          </select>
        </Field>

        <Field label="Custom CSS">
          <textarea
            rows={5}
            spellcheck={false}
            value={s.customCss}
            onInput={(e) => set({ customCss: e.currentTarget.value })}
          />
        </Field>
        <ThemeChecks findings={cssFindings} />

        <div class="hp-field">
          <span>Widgets</span>
          {config.widgets.length > 0 && (
            <ul class="hp-connect-list">
              {config.widgets.map((w) => (
                <li key={w.id}>
                  <span>
                    <strong>{w.title || WIDGET_TYPES[w.type]?.name || w.type}</strong>
                  </span>
                  <select
                    aria-label={`Where ${w.title || WIDGET_TYPES[w.type]?.name || w.type} goes`}
                    value={w.area ?? "header"}
                    onChange={(e) =>
                      onChange({
                        ...config,
                        widgets: config.widgets.map((x) => (x.id === w.id ? { ...x, area: e.currentTarget.value } : x)),
                      })
                    }
                  >
                    <option value="header">Top</option>
                    {groups.map((g) => (
                      <option value={g.id}>In {g.name}</option>
                    ))}
                    <option value="sidebar">Bottom</option>
                  </select>
                  <button
                    type="button"
                    class="hp-button hp-button--quiet"
                    onClick={() => onChange({ ...config, widgets: config.widgets.filter((x) => x.id !== w.id) })}
                  >
                    Remove
                  </button>
                </li>
              ))}
            </ul>
          )}
          <select
            value=""
            onChange={(e) => {
              const type = e.currentTarget.value;
              if (!type) return;
              onChange({ ...config, widgets: [...config.widgets, newWidget(type, config.widgets.map((w) => w.id))] });
            }}
          >
            <option value="">Add a widget…</option>
            {Object.entries(WIDGET_TYPES).map(([type, t]) => (
              <option value={type}>{t.name}</option>
            ))}
          </select>
        </div>

        {apps.length > 0 && (
          <div class="hp-field">
            <span>Apps</span>
            <ul class="hp-connect-list">
              {apps.map((a) => (
                <li key={a.service}>
                  <span>
                    <strong>{a.name}</strong>
                    <small>{a.connected ? "Connected" : KIND_NAMES[a.kind]}</small>
                  </span>
                  <button type="button" class="hp-button hp-button--quiet" onClick={() => onManageApp(a.service)}>
                    {a.connected ? "Manage" : "Connect"}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}

        {apps.length > 0 && permission !== "unsupported" && (
          <div class="hp-field">
            <span>Desktop notifications</span>
            {permission === "granted" ? (
              <select
                value={s.desktopNotify === false ? "off" : "on"}
                onChange={(e) => set({ desktopNotify: e.currentTarget.value === "on" })}
              >
                <option value="on">On, while this tab is in the background</option>
                <option value="off">Off</option>
              </select>
            ) : permission === "denied" ? (
              <small class="hp-dialog-help">Blocked in this browser's site settings.</small>
            ) : (
              <button
                type="button"
                class="hp-button hp-button--quiet"
                onClick={async () => setPermission(await Notification.requestPermission())}
              >
                Allow notifications
              </button>
            )}
          </div>
        )}

        {error && <p class="hp-error">{error}</p>}

        <div class="hp-drawer-actions">
          <button type="button" class="hp-button" onClick={onExport}>
            Export haepage.json
          </button>
          <label class="hp-button">
            Import…
            <input
              type="file"
              accept="application/json,.json"
              hidden
              onChange={(e) => {
                const file = e.currentTarget.files?.[0];
                e.currentTarget.value = "";
                if (file) onImport(file);
              }}
            />
          </label>
        </div>
        <p class="hp-drawer-note">
          {serverSaves
            ? "Changes are saved on the server, for every browser and device."
            : "Changes are saved in this browser. Export haepage.json and put it next to index.html to keep them for everyone."}
        </p>
        <p class="hp-drawer-note">
          Haepage is open source under the{" "}
          <a href="./license.txt" target="_blank" rel="noopener">
            Apache License 2.0
          </a>{" "}
          ·{" "}
          <a href="./third-party-notices.txt" target="_blank" rel="noopener">
            Third-party licenses
          </a>
        </p>
      </aside>
    </div>
  );
}

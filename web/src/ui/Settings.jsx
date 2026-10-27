import { useEffect, useRef, useState } from "preact/hooks";
import { BUILTIN_THEMES, isThemeUrl } from "../theme/apply.js";

function Field({ label, children }) {
  return (
    <label class="hp-field">
      <span>{label}</span>
      {children}
    </label>
  );
}

export function Settings({ config, engines, serverSaves, error, onChange, onExport, onImport, onClose }) {
  const s = config.settings;
  const set = (patch) => onChange({ ...config, settings: { ...s, ...patch } });
  const customTheme = isThemeUrl(s.theme);
  const [showUrl, setShowUrl] = useState(customTheme);
  const panel = useRef(null);

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

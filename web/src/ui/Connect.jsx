import { useEffect, useRef, useState } from "preact/hooks";
import { KIND_NAMES } from "../integration.js";

// Where to find the API key, and how to set up the webhook, per app.
const GUIDE = {
  jellyfin: {
    key: "In Jellyfin, open Dashboard → API Keys, choose + (Add), name it Haepage, and paste the key here.",
  },
  seerr: {
    key: "In Jellyseerr or Overseerr, open Settings → General and copy the API Key.",
    webhook:
      "For instant alerts: Settings → Notifications → Webhook. Turn it on, paste this URL as the Webhook URL, keep the default JSON payload, and tick Request Pending Approval, Request Approved and Request Available.",
  },
  radarr: {
    key: "In Radarr, open Settings → General and copy the API Key.",
    webhook: "For instant alerts: Settings → Connect → + → Webhook. Paste this URL and tick On Grab, On File Import and On Health Issue.",
  },
  sonarr: {
    key: "In Sonarr, open Settings → General and copy the API Key.",
    webhook: "For instant alerts: Settings → Connect → + → Webhook. Paste this URL and tick On Grab, On File Import and On Health Issue.",
  },
};

const sentence = (s) => (s ? s[0].toUpperCase() + s.slice(1) : s);

async function post(path, body) {
  const res = await fetch(path, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(sentence(data.error ?? `HTTP ${res.status}`));
  return data;
}

export function ConnectDialog({ app, onClose, onChanged }) {
  const [key, setKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);
  const [copied, setCopied] = useState(false);
  const input = useRef(null);
  const guide = GUIDE[app.kind] ?? {};
  const webhookURL = app.webhook ? location.origin + app.webhook : null;

  useEffect(() => {
    input.current?.focus();
    const onKey = (e) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  const connect = async (e) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await post("./api/integrations/connect", { service: app.service, key });
      setKey("");
      await onChanged();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const disconnect = async () => {
    setBusy(true);
    try {
      await post("./api/integrations/disconnect", { service: app.service });
      await onChanged();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(webhookURL);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      input.current?.select();
    }
  };

  return (
    <div class="hp-drawer-backdrop hp-dialog-backdrop" onClick={onClose}>
      <div class="hp-dialog" role="dialog" aria-modal="true" aria-label={`Connect ${app.name}`} onClick={(e) => e.stopPropagation()}>
        <div class="hp-drawer-head">
          <h2>
            {app.connected ? "" : "Connect "}
            {app.name}
          </h2>
          <button type="button" class="hp-icon-button" onClick={onClose} aria-label="Close">
            ✕
          </button>
        </div>
        <p class="hp-drawer-note">{KIND_NAMES[app.kind]}</p>

        {app.connected ? (
          <>
            <p class="hp-dialog-ok">Connected. Live data shows on the dashboard.</p>
            {webhookURL && guide.webhook && (
              <div class="hp-field">
                <span>Webhook URL</span>
                <div class="hp-copy">
                  <input ref={input} readOnly value={webhookURL} onFocus={(e) => e.currentTarget.select()} />
                  <button type="button" class="hp-button hp-button--quiet" onClick={copy}>
                    {copied ? "Copied" : "Copy"}
                  </button>
                </div>
                <small class="hp-dialog-help">{guide.webhook}</small>
              </div>
            )}
            {error && <p class="hp-error">{error}</p>}
            <div class="hp-drawer-actions">
              <button type="button" class="hp-button" onClick={onClose}>
                Done
              </button>
              <button type="button" class="hp-button hp-button--quiet" disabled={busy} onClick={disconnect}>
                Disconnect
              </button>
            </div>
          </>
        ) : (
          <form onSubmit={connect}>
            <label class="hp-field">
              <span>API key</span>
              <input
                ref={input}
                type="password"
                autocomplete="off"
                spellcheck={false}
                value={key}
                onInput={(e) => setKey(e.currentTarget.value)}
              />
              <small class="hp-dialog-help">{guide.key}</small>
            </label>
            {error && <p class="hp-error">{error}</p>}
            <div class="hp-drawer-actions">
              <button type="submit" class="hp-button" disabled={busy || !key.trim()}>
                {busy ? "Checking…" : "Connect"}
              </button>
              <button type="button" class="hp-button hp-button--quiet" onClick={onClose}>
                Cancel
              </button>
            </div>
            <p class="hp-drawer-note">The key is checked with the app, then kept on your Haepage server only. It is never sent to browsers.</p>
          </form>
        )}
      </div>
    </div>
  );
}

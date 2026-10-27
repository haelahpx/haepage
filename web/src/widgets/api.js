// Helpers shared by widget modules (docs/widget-api.md).
import { useEffect, useRef, useState } from "preact/hooks";

// Calls fn now and every `seconds` while the tab is visible, and again as
// soon as it becomes visible. Hidden tabs do no work.
export function useEvery(fn, seconds) {
  const latest = useRef(fn);
  latest.current = fn;
  useEffect(() => {
    if (!seconds) return undefined;
    const run = () => !document.hidden && latest.current();
    run();
    const timer = setInterval(run, seconds * 1000);
    document.addEventListener("visibilitychange", run);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", run);
    };
  }, [seconds]);
}

// A safe link for user-entered addresses: http(s) only.
export function safeUrl(url) {
  try {
    const u = new URL(url);
    return u.protocol === "http:" || u.protocol === "https:" ? u.href : null;
  } catch {
    return null;
  }
}

// Loads a widget's data through the server (/api/widgets/{id}/data) every
// `seconds` while the tab is visible, and again shortly after its settings
// change (they save a moment after an edit). `settingsKey` names what the
// server reads, so other edits don't refetch.
export function useWidgetData(widget, enabled, settingsKey, seconds = widget.refresh ?? 300) {
  const [data, setData] = useState(undefined);
  const [error, setError] = useState(null);
  const load = async () => {
    try {
      const res = await fetch(`./api/widgets/${encodeURIComponent(widget.id)}/data`, { cache: "no-store" });
      const body = await res.json().catch(() => null);
      if (!res.ok) throw new Error(body?.error ?? `HTTP ${res.status}`);
      setData(body);
      setError(null);
    } catch (err) {
      setError(err.message);
    }
  };
  useEvery(load, enabled ? seconds : 0);
  useEffect(() => {
    if (!enabled) return undefined;
    const t = setTimeout(load, 900);
    return () => clearTimeout(t);
  }, [settingsKey]);
  return { data, error };
}

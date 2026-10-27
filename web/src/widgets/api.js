// Helpers shared by widget modules (docs/widget-api.md).
import { useEffect, useRef } from "preact/hooks";

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

import { render } from "preact";
import { App } from "./app.jsx";
import { applyCachedTheme } from "./theme/apply.js";
import { trackPresses } from "./theme/press.js";

// Before the first render, so the last-used theme paints straight away.
applyCachedTheme();
document.documentElement.dataset.scheme = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";

trackPresses();

// Offline support and installing as an app. Browsers allow service workers
// only on https:// and localhost.
if ("serviceWorker" in navigator && window.isSecureContext) {
  addEventListener("load", () => navigator.serviceWorker.register("./sw.js").catch(() => {}));
}
render(<App />, document.getElementById("app"));

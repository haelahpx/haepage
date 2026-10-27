import { render } from "preact";
import { App } from "./app.jsx";
import { applyCachedTheme } from "./theme/apply.js";
import { trackPresses } from "./theme/press.js";

// Before the first render, so the last-used theme paints straight away.
applyCachedTheme();
document.documentElement.dataset.scheme = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";

trackPresses();
render(<App />, document.getElementById("app"));

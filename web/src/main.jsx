import { render } from "preact";
import { useEffect, useState } from "preact/hooks";
import { detectMode } from "./mode.js";

const root = document.documentElement;
const dark = window.matchMedia("(prefers-color-scheme: dark)");

function applyScheme() {
  root.dataset.scheme = dark.matches ? "dark" : "light";
}

function App() {
  const [info, setInfo] = useState(null);

  useEffect(() => {
    detectMode().then((result) => {
      root.dataset.mode = result.mode;
      setInfo(result);
    });
  }, []);

  let status = "Starting…";
  if (info?.mode === "server") status = `Server mode · ${info.version}`;
  else if (info) status = "Static mode";

  return (
    <main class="hp-app">
      <header class="hp-header">
        <h1 class="hp-title">Haepage</h1>
      </header>
      <p class="hp-empty">{status}</p>
    </main>
  );
}

applyScheme();
dark.addEventListener("change", applyScheme);
render(<App />, document.getElementById("app"));

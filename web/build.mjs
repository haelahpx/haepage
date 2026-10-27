// Builds the frontend into web/dist: bundles the JS, inlines critical CSS
// into index.html, copies the themes, precompresses everything, and fails
// when a size budget is exceeded.
import * as esbuild from "esbuild";
import { copyFile, mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { basename, dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { brotliCompressSync, gzipSync } from "node:zlib";

const web = dirname(fileURLToPath(import.meta.url));
const root = join(web, "..");
const dist = join(web, "dist");

// Gzipped byte limits (IMPLEMENTATION.md §6.7). Warn above `warn`, fail above `fail`.
const budgets = {
  js: { warn: 50 * 1024, fail: 60 * 1024 },
  html: { warn: 14 * 1024, fail: 14 * 1024 },
  css: { warn: 15 * 1024, fail: 50 * 1024 },
  chunk: { warn: 10 * 1024, fail: 15 * 1024 },
};

const compressible = /\.(js|css|html|json|svg|webmanifest)$/;

async function clean() {
  // Keep .gitkeep: go:embed needs web/dist to exist on a fresh clone.
  for (const name of await readdir(dist).catch(() => [])) {
    if (name !== ".gitkeep") await rm(join(dist, name), { recursive: true });
  }
  await mkdir(join(dist, "assets"), { recursive: true });
}

async function bundle() {
  const result = await esbuild.build({
    entryPoints: [join(web, "src/main.jsx")],
    bundle: true,
    minify: true,
    format: "esm",
    target: "es2020",
    jsx: "automatic",
    jsxImportSource: "preact",
    entryNames: "[name]-[hash]",
    chunkNames: "[name]-[hash]",
    splitting: true,
    outdir: join(dist, "assets"),
    metafile: true,
    logLevel: "warning",
  });
  // The page's entry chunk. Lazily imported widgets count as entry points
  // too, so match main.jsx itself.
  const out = Object.entries(result.metafile.outputs).find(([, o]) => o.entryPoint?.endsWith("src/main.jsx"))[0];
  return basename(out);
}

async function minifyCss(css) {
  return (await esbuild.transform(css, { loader: "css", minify: true })).code;
}

// Inlines base.css and the default theme, with separate <style> elements so
// the app can swap the theme and custom CSS without touching the base.
// Paths are relative so the page also works from a sub-folder.
async function page(jsName) {
  const base = await readFile(join(root, "themes/base.css"), "utf8");
  const theme = await readFile(join(root, "themes/tonal/style.css"), "utf8");
  const css =
    `<style>${await minifyCss(base)}</style>` +
    `<style id="hp-theme">${await minifyCss(theme)}</style>` +
    `<style id="hp-custom"></style>`;
  const html = (await readFile(join(web, "public/index.html"), "utf8"))
    .replace("<!--hp:css-->", css)
    .replace("<!--hp:js-->", `<script type="module" src="./assets/${jsName}"></script>`);
  await writeFile(join(dist, "index.html"), html);
}

async function themes() {
  await mkdir(join(dist, "themes"), { recursive: true });
  for (const entry of await readdir(join(root, "themes"), { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const dir = join(root, "themes", entry.name);
    const css = await readFile(join(dir, "style.css"), "utf8");
    await writeFile(join(dist, "themes", `${entry.name}.css`), await minifyCss(css));
    await writeFile(join(dist, "themes", `${entry.name}.json`), await readFile(join(dir, "theme.json")));
  }
}

// Haepage's license and the notices for bundled third-party code ship with
// every build (the static bundle and the binary both serve web/dist).
async function notices() {
  await writeFile(join(dist, "license.txt"), await readFile(join(root, "LICENSE")));
  await writeFile(join(dist, "third-party-notices.txt"), await readFile(join(root, "THIRD_PARTY_NOTICES.md")));
}

// The manifest and icons for installing Haepage as an app.
async function pwaFiles() {
  await mkdir(join(dist, "icons"), { recursive: true });
  await copyFile(join(web, "public/manifest.webmanifest"), join(dist, "manifest.webmanifest"));
  for (const name of await readdir(join(web, "public/icons"))) {
    await copyFile(join(web, "public/icons", name), join(dist, "icons", name));
  }
}

// The service worker precaches the app shell. Its version is a hash of the
// shell, so any change to the build installs a new worker.
async function serviceWorker(jsName) {
  const shell = [
    "./",
    ...(await readdir(join(dist, "assets"))).filter((f) => f.endsWith(".js")).map((f) => `./assets/${f}`),
    "./manifest.webmanifest",
    ...(await readdir(join(dist, "themes"))).filter((f) => f.endsWith(".css")).map((f) => `./themes/${f}`),
    ...(await readdir(join(dist, "icons"))).map((f) => `./icons/${f}`),
  ];
  const hash = createHash("sha256");
  for (const file of [...shell.slice(1), "./index.html"]) hash.update(await readFile(join(dist, file)));
  const sw = (await readFile(join(web, "public/sw.js"), "utf8"))
    .replace("__VERSION__", hash.digest("hex").slice(0, 12))
    .replace("__SHELL__", JSON.stringify(shell));
  await writeFile(join(dist, "sw.js"), (await esbuild.transform(sw, { minify: true })).code);
}

async function walk(dir) {
  const files = [];
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) files.push(...(await walk(path)));
    else files.push(path);
  }
  return files;
}

// Writes .gz and .br next to each text file and returns the gzipped sizes.
async function compress() {
  const sizes = [];
  for (const file of await walk(dist)) {
    if (!compressible.test(file)) continue;
    const data = await readFile(file);
    const gz = gzipSync(data, { level: 9 });
    await writeFile(`${file}.gz`, gz);
    await writeFile(`${file}.br`, brotliCompressSync(data));
    sizes.push({ file: relative(dist, file), raw: data.length, gz: gz.length });
  }
  return sizes;
}

function check(sizes, jsName) {
  let failed = false;
  const kb = (n) => `${(n / 1024).toFixed(1)} KB`;
  for (const { file, raw, gz } of sizes) {
    const kind =
      file === `assets/${jsName}`
        ? "js"
        : file.startsWith("assets/") && file.endsWith(".js")
          ? "chunk"
          : file === "index.html"
            ? "html"
            : file.endsWith(".css")
              ? "css"
              : null;
    const budget = kind && budgets[kind];
    let note = "";
    if (budget && gz > budget.fail) {
      note = `  FAIL (budget ${kb(budget.fail)})`;
      failed = true;
    } else if (budget && gz > budget.warn) {
      note = `  warn (target ${kb(budget.warn)})`;
    }
    console.log(`${file.padEnd(32)} ${kb(raw).padStart(9)}  gz ${kb(gz).padStart(9)}${note}`);
  }
  if (failed) {
    console.error("\nSize budget exceeded.");
    process.exit(1);
  }
}

await clean();
const jsName = await bundle();
await page(jsName);
await themes();
await notices();
await pwaFiles();
await serviceWorker(jsName);
check(await compress(), jsName);

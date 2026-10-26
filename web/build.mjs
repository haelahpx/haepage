// Builds the frontend into web/dist: bundles the JS, inlines critical CSS
// into index.html, copies the themes, precompresses everything, and fails
// when a size budget is exceeded.
import * as esbuild from "esbuild";
import { mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
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
};

const compressible = /\.(js|css|html|json|svg)$/;

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
    outdir: join(dist, "assets"),
    metafile: true,
    logLevel: "warning",
  });
  const out = Object.keys(result.metafile.outputs).find((f) => f.endsWith(".js"));
  return basename(out);
}

async function minifyCss(css) {
  return (await esbuild.transform(css, { loader: "css", minify: true })).code;
}

async function page(jsName) {
  const css =
    (await readFile(join(root, "themes/base.css"), "utf8")) +
    (await readFile(join(root, "themes/android/style.css"), "utf8"));
  const html = (await readFile(join(web, "public/index.html"), "utf8"))
    .replace("<!--hp:css-->", `<style>${await minifyCss(css)}</style>`)
    .replace("<!--hp:js-->", `<script type="module" src="/assets/${jsName}"></script>`);
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
    const kind = file === `assets/${jsName}` ? "js" : file === "index.html" ? "html" : file.endsWith(".css") ? "css" : null;
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
check(await compress(), jsName);

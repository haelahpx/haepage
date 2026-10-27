import assert from "node:assert/strict";
import { test } from "node:test";
import { applyWallpaper, pickWidth, wallpaperUrls } from "./wallpaper.js";

test("pickWidth", () => {
  assert.equal(pickWidth([1280, 1920, 2560], 390, 3), 1280);
  assert.equal(pickWidth([1280, 1920, 2560], 1440, 1), 1920);
  assert.equal(pickWidth([1280, 1920, 2560], 1440, 2), 2560);
  assert.equal(pickWidth([1280, 1500], 2560, 2), 1500);
  assert.equal(pickWidth([0], 1920, 1), 0);
});

const list = [
  { name: "sky.jpg", hash: "abc", sizes: [1280, 1920, 2560], blurred: true },
  { name: "photo.webp", hash: "def", sizes: [0], blurred: false },
];

test("wallpaperUrls resolves folder names, links and paths", () => {
  assert.deepEqual(wallpaperUrls("sky.jpg", list, { screenWidth: 1440, dpr: 1 }), {
    wallpaper: "./wallpapers/abc/1920",
    blurred: "./wallpapers/abc/blur",
  });
  assert.deepEqual(wallpaperUrls("photo.webp", list), { wallpaper: "./wallpapers/def/0", blurred: null });
  assert.deepEqual(wallpaperUrls("https://img.example/a.jpg"), { wallpaper: "https://img.example/a.jpg", blurred: null });
  assert.deepEqual(wallpaperUrls("backgrounds/hills.png"), { wallpaper: "backgrounds/hills.png", blurred: null });
  assert.equal(wallpaperUrls(""), null);
  // Static mode: a file next to index.html.
  assert.deepEqual(wallpaperUrls("sky.jpg"), { wallpaper: "sky.jpg", blurred: null });
});

test("wallpaperUrls refuses non-images and odd schemes", () => {
  for (const bad of ["javascript:alert(1)", "http://img.example/a.jpg", "//img.example/a.jpg", "notes.txt", "data:image/png;base64,AA"]) {
    assert.equal(wallpaperUrls(bad), null, bad);
  }
});

test("applyWallpaper sets and clears variables and attributes", () => {
  const props = {};
  const root = {
    dataset: {},
    style: { setProperty: (k, v) => (props[k] = v), removeProperty: (k) => delete props[k] },
  };
  applyWallpaper({ wallpaper: './w/a"b', blurred: "./w/blur" }, root);
  assert.equal(props["--wallpaper"], 'url("./w/a\\"b")');
  assert.equal(props["--wallpaper-blurred"], 'url("./w/blur")');
  assert.ok("wallpaper" in root.dataset && "wallpaperBlurred" in root.dataset);
  applyWallpaper({ wallpaper: "./w/a", blurred: null }, root);
  assert.ok(!("--wallpaper-blurred" in props) && !("wallpaperBlurred" in root.dataset));
  applyWallpaper(null, root);
  assert.deepEqual(props, {});
  assert.deepEqual(root.dataset, {});
});

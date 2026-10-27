// Wallpapers: settings.wallpaper names a file in the server's wallpaper
// folder (Server mode) or is an image link or path (either mode). The page
// gets --wallpaper and, when the server made one, --wallpaper-blurred, plus
// data-wallpaper and data-wallpaper-blurred on <html> for themes to key on.

const IMAGE_PATH = /^(https:\/\/|\.{0,2}\/|[\w-][\w./-]*\.(jpe?g|png|gif|webp|avif)$)/i;

// The smallest copy at least as wide as the screen in device pixels, or the
// largest there is.
export function pickWidth(sizes, screenWidth, dpr = 1) {
  const need = screenWidth * dpr;
  return sizes.find((w) => w >= need) ?? sizes[sizes.length - 1];
}

const cssUrl = (u) => `url(${JSON.stringify(u)})`;

// Resolves settings.wallpaper to image URLs, or null for none.
export function wallpaperUrls(setting, list = [], { screenWidth = 1920, dpr = 1 } = {}) {
  if (!setting) return null;
  const entry = list.find((w) => w.name === setting);
  if (entry) {
    const base = `./wallpapers/${entry.hash}/`;
    return {
      wallpaper: base + pickWidth(entry.sizes, screenWidth, dpr),
      blurred: entry.blurred ? base + "blur" : null,
    };
  }
  return IMAGE_PATH.test(setting) && !setting.startsWith("//") ? { wallpaper: setting, blurred: null } : null;
}

export function applyWallpaper(urls, root = document.documentElement) {
  const set = (name, value) => (value ? root.style.setProperty(name, value) : root.style.removeProperty(name));
  set("--wallpaper", urls && cssUrl(urls.wallpaper));
  set("--wallpaper-blurred", urls?.blurred && cssUrl(urls.blurred));
  if (urls) root.dataset.wallpaper = "";
  else delete root.dataset.wallpaper;
  if (urls?.blurred) root.dataset.wallpaperBlurred = "";
  else delete root.dataset.wallpaperBlurred;
}

export async function listWallpapers(fetchFn = fetch) {
  const res = await fetchFn("./api/wallpapers");
  if (!res.ok) throw new Error(`Wallpapers could not be listed (HTTP ${res.status})`);
  return res.json();
}

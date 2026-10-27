// Haepage service worker (IMPLEMENTATION.md M3.7). The build fills in
// VERSION and SHELL. Strategy:
// - the page and data (config, services, ping…): network first, so changes
//   show at once; the last good copy is used when the server can't be
//   reached, marked with an x-haepage-offline header;
// - hashed assets and wallpaper copies: cache first (their URLs change when
//   their content does);
// - writes, the event stream and other sites: not touched.
const VERSION = "__VERSION__";
const SHELL = __SHELL__;
const SHELL_CACHE = `haepage-shell-${VERSION}`;
const DATA_CACHE = "haepage-data";
const IMAGE_CACHE = "haepage-images";
const IMAGE_LIMIT = 12;
const NETWORK_TIMEOUT = 4000;

const scope = new URL(self.registration.scope);
const local = (url) => url.origin === scope.origin && url.pathname.startsWith(scope.pathname);
const path = (url) => url.pathname.slice(scope.pathname.length);

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(SHELL_CACHE)
      .then((cache) => cache.addAll(SHELL))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k.startsWith("haepage-shell-") && k !== SHELL_CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

// Network data that is worth keeping for offline use.
const DATA = /^(api\/(ping|config|services|system|integrations|wallpapers|themes\/remote)|haepage\.json)$/;

self.addEventListener("fetch", (event) => {
  const req = event.request;
  const url = new URL(req.url);
  if (req.method !== "GET" || !local(url) || req.headers.get("accept") === "text/event-stream") return;
  const p = path(url);

  if (req.mode === "navigate") {
    event.respondWith(networkFirst(req, SHELL_CACHE, "./"));
  } else if (DATA.test(p)) {
    event.respondWith(networkFirst(req, DATA_CACHE));
  } else if (p.startsWith("assets/") || p.startsWith("icons/")) {
    event.respondWith(cacheFirst(req, SHELL_CACHE));
  } else if (p.startsWith("wallpapers/")) {
    event.respondWith(cacheFirst(req, IMAGE_CACHE, IMAGE_LIMIT));
  } else if (p.startsWith("themes/")) {
    event.respondWith(networkFirst(req, SHELL_CACHE));
  }
});

// A failed request or a server error (say, a proxy whose backend is down)
// falls back to the last good copy.
async function networkFirst(req, cacheName, fallbackKey) {
  const cache = await caches.open(cacheName);
  let res;
  try {
    res = await fetch(req, { signal: AbortSignal.timeout(NETWORK_TIMEOUT) });
    if (res.ok) {
      cache.put(fallbackKey ?? req, res.clone());
      return res;
    }
    if (res.status < 500) return res;
  } catch (err) {
    res = err;
  }
  const hit = (await cache.match(fallbackKey ?? req, { ignoreSearch: !!fallbackKey })) ?? (await caches.match(req));
  if (!hit) {
    if (res instanceof Response) return res;
    throw res;
  }
  const headers = new Headers(hit.headers);
  headers.set("x-haepage-offline", "1");
  return new Response(hit.body, { status: hit.status, statusText: hit.statusText, headers });
}

async function cacheFirst(req, cacheName, limit) {
  const cache = await caches.open(cacheName);
  const hit = await cache.match(req);
  if (hit) return hit;
  const res = await fetch(req);
  if (res.ok) {
    await cache.put(req, res.clone());
    if (limit) {
      const keys = await cache.keys();
      await Promise.all(keys.slice(0, Math.max(0, keys.length - limit)).map((k) => cache.delete(k)));
    }
  }
  return res;
}

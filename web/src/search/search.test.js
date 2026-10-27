import assert from "node:assert/strict";
import { test } from "node:test";
import { BUILTIN_ENGINES, allEngines, findEngine, parseQuery, searchUrl } from "./engines.js";
import { buildIndex, search } from "./index.js";

const services = [
  { id: "media", name: "Media Server", group: "media", urls: { lan: "http://192.168.1.10:8096" } },
  { id: "photos", name: "Photo Library", group: "media", tags: ["pictures"], urls: { lan: "http://192.168.1.11" } },
  { id: "router", name: "Router", group: "net", description: "Network admin", urls: { lan: "http://192.168.1.1" } },
  { id: "nourl", name: "Media Notes", urls: {} },
];
const groups = [
  { id: "media", name: "Media" },
  { id: "net", name: "Network" },
];
const bookmarks = [{ name: "Mail", url: "https://mail.example.com" }, { name: "" }];
const entries = buildIndex({ services, bookmarks, groups, hostname: "192.168.1.2" });
const names = (results) => results.map((r) => r.name);

test("index skips services without a URL and bad bookmarks", () => {
  assert.deepEqual(names(entries), ["Media Server", "Photo Library", "Router", "Mail"]);
});

test("name prefix beats word prefix", () => {
  assert.deepEqual(names(search(entries, "li")), ["Photo Library"]);
  assert.deepEqual(names(search(entries, "r")).slice(0, 2), ["Router", "Media Server"]);
});

test("matches group names, tags, descriptions and hosts", () => {
  assert.deepEqual(names(search(entries, "media")), ["Media Server", "Photo Library"]);
  assert.deepEqual(names(search(entries, "pictures")), ["Photo Library"]);
  assert.deepEqual(names(search(entries, "admin")), ["Router"]);
  assert.deepEqual(names(search(entries, "mail.example")), ["Mail"]);
});

test("in-order letters match as a last resort", () => {
  assert.deepEqual(names(search(entries, "mdsvr")), ["Media Server"]);
});

test("every word must match", () => {
  assert.deepEqual(names(search(entries, "media photo")), ["Photo Library"]);
  assert.deepEqual(search(entries, "media zzz"), []);
});

test("empty query and limit", () => {
  assert.deepEqual(search(entries, "   "), []);
  assert.equal(search(entries, "e", 2).length, 2);
});

test("bangs at the start or end pick an engine", () => {
  const engines = allEngines();
  let q = parseQuery("!g cats", engines);
  assert.equal(q.engine.id, "google");
  assert.equal(q.terms, "cats");
  q = parseQuery("cute cats !YT", engines);
  assert.equal(q.engine.id, "youtube");
  assert.equal(q.terms, "cute cats");
  q = parseQuery("!w", engines);
  assert.equal(q.engine.id, "wikipedia");
  assert.equal(q.terms, "");
});

test("unknown bangs are plain text", () => {
  const q = parseQuery("!nope cats", allEngines());
  assert.equal(q.bang, false);
  assert.equal(q.terms, "!nope cats");
});

test("searchUrl encodes terms and opens the home page when empty", () => {
  const ddg = findEngine(allEngines(), "duckduckgo");
  assert.equal(searchUrl(ddg, "a&b c"), "https://duckduckgo.com/?q=a%26b%20c");
  assert.equal(searchUrl(ddg, ""), "https://duckduckgo.com");
});

test("config engines add to and override built-ins", () => {
  const engines = allEngines([
    { id: "google", prefix: "gg", url: "https://www.google.com/search?q=%s" },
    { id: "wiki-de", name: "Wikipedia DE", prefix: "wde", url: "https://de.wikipedia.org/w/index.php?search=%s" },
    { id: "broken", prefix: "x", url: "https://example.com/" },
  ]);
  assert.equal(engines.length, BUILTIN_ENGINES.length + 1);
  const google = findEngine(engines, "google");
  assert.equal(google.prefix, "gg");
  assert.equal(google.name, "Google");
  assert.equal(parseQuery("!wde Berlin", engines).engine.id, "wiki-de");
});

test("findEngine falls back to the first engine", () => {
  assert.equal(findEngine(allEngines(), "missing").id, "duckduckgo");
});

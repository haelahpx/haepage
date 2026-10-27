import assert from "node:assert/strict";
import { test } from "node:test";
import { fetchThemeCss, importTheme, shareLink, themeFromLink } from "./apply.js";

const reply = (status, body) => ({
  ok: status >= 200 && status < 300,
  status,
  text: async () => body,
  json: async () => JSON.parse(body),
});

test("themeFromLink accepts only https links", () => {
  assert.equal(themeFromLink("?theme=https%3A%2F%2Fa.example%2Ft.css"), "https://a.example/t.css");
  assert.equal(themeFromLink("?theme=http://a.example/t.css"), null);
  assert.equal(themeFromLink("?theme=javascript:alert(1)"), null);
  assert.equal(themeFromLink("?theme=glass"), null);
  assert.equal(themeFromLink(""), null);
});

test("shareLink round-trips through themeFromLink", () => {
  const link = shareLink("https://a.example/t.css?v=2&x=1", { origin: "https://home.example", pathname: "/dash/" });
  assert.equal(link, "https://home.example/dash/?theme=https%3A%2F%2Fa.example%2Ft.css%3Fv%3D2%26x%3D1");
  assert.equal(themeFromLink(new URL(link).search), "https://a.example/t.css?v=2&x=1");
});

test("server mode imports a link theme on first use", async () => {
  const calls = [];
  let imported = false;
  const fetchFn = async (url, opts) => {
    calls.push(opts?.method ?? "GET");
    if (url.startsWith("./api/themes/import")) {
      imported = true;
      assert.deepEqual(JSON.parse(opts.body), { url: "https://a.example/t.css", refresh: false });
      return reply(200, '{"sha256":"x"}');
    }
    return imported ? reply(200, ":root{}") : reply(404, '{"error":"theme not imported"}');
  };
  assert.equal(await fetchThemeCss("https://a.example/t.css", { server: true, fetchFn }), ":root{}");
  assert.deepEqual(calls, ["GET", "POST", "GET"]);
});

test("server import errors carry the server's reason", async () => {
  const fetchFn = async () => reply(400, '{"error":"address not allowed: 127.0.0.1 is a local address"}');
  await assert.rejects(importTheme("https://127.0.0.1/t.css", { fetchFn }), /local address/);
});

test("static mode fetches the link in the browser", async () => {
  const seen = [];
  const fetchFn = async (url) => (seen.push(url), reply(200, "x"));
  await fetchThemeCss("https://a.example/t.css", { fetchFn });
  await fetchThemeCss("glass", { fetchFn });
  assert.deepEqual(seen, ["https://a.example/t.css", "./themes/glass.css"]);
  const blocked = async () => {
    throw new TypeError("Failed to fetch");
  };
  await assert.rejects(fetchThemeCss("https://a.example/t.css", { fetchFn: blocked }), /CORS/);
});

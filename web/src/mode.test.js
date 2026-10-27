import assert from "node:assert/strict";
import { test } from "node:test";
import { detectMode } from "./mode.js";

const reply = (status, body) => async () => ({
  ok: status >= 200 && status < 300,
  json: async () => (typeof body === "string" ? JSON.parse(body) : body),
});

test("server mode when /api/ping answers as haepage", async () => {
  const got = await detectMode(reply(200, { app: "haepage", version: "1.0.0", capabilities: ["events"] }));
  assert.equal(got.mode, "server");
  assert.equal(got.version, "1.0.0");
  assert.ok(got.capabilities.has("events"));
});

test("missing capabilities means none", async () => {
  const got = await detectMode(reply(200, { app: "haepage", version: "1.0.0" }));
  assert.equal(got.capabilities.size, 0);
});

test("static mode on HTTP error", async () => {
  assert.equal((await detectMode(reply(404, {}))).mode, "static");
});

test("static mode when another app answers", async () => {
  assert.equal((await detectMode(reply(200, { app: "other" }))).mode, "static");
});

test("static mode on a non-JSON body", async () => {
  assert.equal((await detectMode(reply(200, "<html>"))).mode, "static");
});

test("static mode on network error", async () => {
  const fail = async () => {
    throw new TypeError("network");
  };
  assert.equal((await detectMode(fail)).mode, "static");
});

test("a ping answered from the service worker cache means offline", async () => {
  const cached = async (url) => {
    assert.equal(url, "./api/ping");
    return { ok: true, headers: new Headers({ "x-haepage-offline": "1" }), json: async () => ({ app: "haepage", capabilities: [] }) };
  };
  const got = await detectMode(cached);
  assert.equal(got.mode, "server");
  assert.equal(got.offline, true);
  assert.equal((await detectMode(reply(200, { app: "haepage" }))).offline, false);
});

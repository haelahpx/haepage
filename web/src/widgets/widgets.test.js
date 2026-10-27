import assert from "node:assert/strict";
import { test } from "node:test";
import { addItem, clockParts } from "./lib.js";

test("clockParts formats any zone", () => {
  const d = new Date("2026-10-27T12:00:00Z");
  const tokyo = clockParts(d, "Asia/Tokyo", "en-GB");
  assert.equal(tokyo.time, "21:00");
  assert.equal(tokyo.city, "Tokyo");
  assert.equal(clockParts(d, "America/New_York", "en-GB").city, "New York");
  assert.equal(clockParts(d, "", "en-GB").city, "");
  assert.throws(() => clockParts(d, "Not/AZone", "en-GB"));
});

test("addItem trims and ignores blanks", () => {
  assert.deepEqual(addItem([], "  milk ", "x"), [{ id: "x", text: "milk", done: false }]);
  const items = [{ id: "a", text: "a", done: false }];
  assert.equal(addItem(items, "   "), items);
});

test("weather helpers", async () => {
  const { describeWeather, forecastUrl, geocodeUrl, dayName } = await import("./lib.js");
  const metric = new URL(forecastUrl({ latitude: -6.2, longitude: 106.8 }));
  assert.equal(metric.host, "api.open-meteo.com");
  assert.equal(metric.searchParams.get("latitude"), "-6.2");
  assert.equal(metric.searchParams.get("temperature_unit"), null);
  const imperial = new URL(forecastUrl({ latitude: 1, longitude: 2, units: "imperial" }));
  assert.equal(imperial.searchParams.get("temperature_unit"), "fahrenheit");
  assert.equal(new URL(geocodeUrl("São Paulo")).searchParams.get("name"), "São Paulo");
  assert.deepEqual(describeWeather(0), { label: "Clear", icon: "sun" });
  assert.equal(describeWeather(63).icon, "rain");
  assert.equal(describeWeather(81).label, "Showers");
  assert.equal(describeWeather(75).icon, "snow");
  assert.equal(describeWeather(96).icon, "storm");
  assert.equal(describeWeather(45).icon, "fog");
  assert.equal(dayName("2026-10-27", "en-GB"), "Tue");
});

test("getPath", async () => {
  const { getPath } = await import("./lib.js");
  const v = { a: { b: [{ c: 5 }, { c: 6 }] }, n: 0 };
  assert.equal(getPath(v, "a.b[1].c"), 6);
  assert.equal(getPath(v, "a.b.0.c"), 5);
  assert.equal(getPath(v, "n"), 0);
  assert.equal(getPath(v, "a.x.y"), undefined);
  assert.equal(getPath(v, "a.b[9].c"), undefined);
  assert.equal(getPath(v, ""), v);
  assert.equal(getPath(null, "a"), undefined);
});

test("formatValue", async () => {
  const { formatValue } = await import("./lib.js");
  assert.equal(formatValue(1234567.891, "number", "en-US"), "1,234,567.89");
  assert.equal(formatValue(0.425, "percent", "en-US"), "42.5%");
  assert.equal(formatValue(87, "percent", "en-US"), "87%");
  assert.equal(formatValue(1536, "bytes", "en-US"), "1.5 KB");
  assert.equal(formatValue(3 * 1024 ** 3, "bytes", "en-US"), "3 GB");
  assert.equal(formatValue(90061, "duration"), "1d 1h");
  assert.equal(formatValue(3700, "duration"), "1h 1m");
  assert.equal(formatValue(42, "duration"), "42s");
  assert.match(formatValue(1_800_000_000, "date", "en-US"), /2027/);
  assert.equal(formatValue(true), "Yes");
  assert.equal(formatValue(null), "—");
  assert.equal(formatValue("abc", "number"), "abc");
  assert.equal(formatValue({ a: 1 }, "text"), '{"a":1}');
});

test("leafPaths and arrayPaths", async () => {
  const { leafPaths, arrayPaths } = await import("./lib.js");
  const v = { status: "ok", stats: { blocked: 7, total: 10 }, clients: [{ name: "tv", hits: 3 }] };
  assert.deepEqual(
    leafPaths(v).map((p) => p.path),
    ["status", "stats.blocked", "stats.total", "clients[0].name", "clients[0].hits"],
  );
  assert.deepEqual(arrayPaths(v), ["clients"]);
  assert.deepEqual(arrayPaths([{ a: [1] }]), ["", "[0].a"]);
  assert.equal(leafPaths({ a: Array.from({ length: 100 }, (_, i) => ({ ["k" + i]: i })) }, 5).length, 1);
});

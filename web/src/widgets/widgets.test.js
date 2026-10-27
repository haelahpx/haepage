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

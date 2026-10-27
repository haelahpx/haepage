import assert from "node:assert/strict";
import { test } from "node:test";
import { barStates, formatUptime } from "./uptime.js";

test("formatUptime", () => {
  assert.equal(formatUptime(100), "100%");
  assert.equal(formatUptime(99.996), "99.99%");
  assert.equal(formatUptime(99.95), "99.95%");
  assert.equal(formatUptime(99.46), "99.4%");
  assert.equal(formatUptime(97.9), "97%");
  assert.equal(formatUptime(0), "0%");
  assert.equal(formatUptime(null), "");
});

test("barStates", () => {
  assert.deepEqual(barStates("udp.x"), ["up", "down", "partial", "none", "none"]);
  assert.deepEqual(barStates(undefined), []);
});

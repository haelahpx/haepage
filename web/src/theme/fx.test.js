import assert from "node:assert/strict";
import { test } from "node:test";
import { autoFx, resolveFx, resolveScheme } from "./fx.js";

test("autoFx", () => {
  assert.equal(autoFx({ deviceMemory: 8, hardwareConcurrency: 8 }), "balanced");
  assert.equal(autoFx({}), "balanced");
  // null, not undefined: undefined would fall back to the real navigator,
  // which depends on the machine running the tests.
  assert.equal(autoFx(null), "balanced");
  assert.equal(autoFx({ deviceMemory: 2 }), "lite");
  assert.equal(autoFx({ hardwareConcurrency: 2 }), "lite");
  assert.equal(autoFx({ connection: { saveData: true } }), "lite");
  assert.equal(autoFx({ deviceMemory: 8 }, true), "lite");
});

test("explicit settings win", () => {
  assert.equal(resolveFx("full", { deviceMemory: 1 }, true), "full");
  assert.equal(resolveFx("auto", { deviceMemory: 1 }), "lite");
  assert.equal(resolveFx("bogus", {}), "balanced");
  assert.equal(resolveScheme("light", true), "light");
  assert.equal(resolveScheme("auto", true), "dark");
  assert.equal(resolveScheme("auto", false), "light");
});

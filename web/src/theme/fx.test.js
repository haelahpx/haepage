import assert from "node:assert/strict";
import { test } from "node:test";
import { FX_KEY, autoFx, measureFrames, readMeasuredFx, resolveFx, resolveScheme, saveMeasuredFx, tierFromFrames } from "./fx.js";

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

test("measured tier is used unless the device is hard-limited", () => {
  assert.equal(autoFx({ deviceMemory: 8 }, false, "full"), "full");
  assert.equal(autoFx({ deviceMemory: 2 }, false, "full"), "lite");
  assert.equal(resolveFx("auto", {}, false, "lite"), "lite");
  assert.equal(resolveFx("balanced", {}, false, "full"), "balanced");
});

test("tierFromFrames", () => {
  assert.equal(tierFromFrames([]), null);
  assert.equal(tierFromFrames(Array(20).fill(8.3)), "full");
  assert.equal(tierFromFrames(Array(20).fill(16.7)), "full");
  assert.equal(tierFromFrames(Array(20).fill(22)), "balanced");
  assert.equal(tierFromFrames(Array(20).fill(33)), "lite");
  // One long frame doesn't move the median.
  assert.equal(tierFromFrames([...Array(19).fill(16.7), 200]), "full");
  assert.equal(tierFromFrames([10, 20, 30, 40]), "balanced");
});

test("measureFrames collects intervals", async () => {
  let t = 0;
  const raf = (cb) => queueMicrotask(() => cb((t += 16)));
  const intervals = await measureFrames(5, raf);
  assert.deepEqual(intervals, [16, 16, 16, 16, 16]);
});

test("measured tier storage", () => {
  const map = new Map();
  const storage = {
    getItem: (k) => map.get(k) ?? null,
    setItem: (k, v) => map.set(k, v),
    removeItem: (k) => map.delete(k),
  };
  assert.equal(readMeasuredFx(storage), null);
  saveMeasuredFx(storage, "full");
  assert.equal(readMeasuredFx(storage), "full");
  storage.setItem(FX_KEY, "bogus");
  assert.equal(readMeasuredFx(storage), null);
  saveMeasuredFx(storage, null);
  assert.equal(map.has(FX_KEY), false);
  const broken = { getItem: () => { throw new Error("blocked"); }, setItem: () => { throw new Error("blocked"); } };
  assert.equal(readMeasuredFx(broken), null);
  saveMeasuredFx(broken, "full");
  assert.equal(readMeasuredFx(null), null);
});

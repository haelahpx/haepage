import assert from "node:assert/strict";
import { test } from "node:test";
import { pressPoint, trackPresses } from "./press.js";

test("pressPoint is relative to the element", () => {
  assert.deepEqual(pressPoint({ left: 100, top: 40 }, 130.4, 52.6), { x: "30px", y: "13px" });
});

test("trackPresses sets variables on the pressed control only", () => {
  let handler;
  const doc = { addEventListener: (_, h) => (handler = h), removeEventListener: () => (handler = null) };
  const props = {};
  const card = {
    getBoundingClientRect: () => ({ left: 10, top: 20 }),
    style: { setProperty: (k, v) => (props[k] = v) },
  };
  const stop = trackPresses(doc);
  handler({ target: { closest: () => card }, clientX: 15, clientY: 30 });
  assert.deepEqual(props, { "--press-x": "5px", "--press-y": "10px" });
  handler({ target: { closest: () => null }, clientX: 0, clientY: 0 });
  handler({ target: {}, clientX: 0, clientY: 0 });
  stop();
  assert.equal(handler, null);
});

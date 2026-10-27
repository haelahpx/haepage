import assert from "node:assert/strict";
import { test } from "node:test";
import { greeting, headline, summarize } from "./status.js";

test("summarize counts by status", () => {
  const got = summarize({ a: { status: "up" }, b: { status: "down" }, c: { status: "unknown" }, d: { status: "up" } });
  assert.deepEqual(got, { total: 4, up: 2, down: 1, unknown: 1 });
  assert.deepEqual(summarize({}), { total: 0, up: 0, down: 0, unknown: 0 });
});

test("headline", () => {
  assert.equal(headline({ total: 0, up: 0, down: 0 }), "No checks yet");
  assert.equal(headline({ total: 3, up: 2, down: 1 }), "1 service down");
  assert.equal(headline({ total: 3, up: 0, down: 2 }), "2 services down");
  assert.equal(headline({ total: 3, up: 3, down: 0 }), "All services up");
  assert.equal(headline({ total: 3, up: 2, down: 0 }), "Everything checked is up");
  assert.equal(headline({ total: 3, up: 0, down: 0 }), "Checking…");
});

test("greeting", () => {
  assert.equal(greeting(2), "Good night");
  assert.equal(greeting(9), "Good morning");
  assert.equal(greeting(15), "Good afternoon");
  assert.equal(greeting(21), "Good evening");
});

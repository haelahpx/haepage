import assert from "node:assert/strict";
import { test } from "node:test";
import { applyEvent, setOverride } from "./events.js";

const a = { id: "a", name: "A" };
const b = { id: "b", name: "B" };

test("snapshot replaces the list", () => {
  assert.deepEqual(applyEvent([a], "snapshot", { services: [b] }), [b]);
  assert.deepEqual(applyEvent([a], "snapshot", {}), []);
});

test("upsert replaces in place or appends", () => {
  assert.deepEqual(applyEvent([a, b], "service.upsert", { id: "a", name: "A2" }), [{ id: "a", name: "A2" }, b]);
  assert.deepEqual(applyEvent([a], "service.upsert", b), [a, b]);
});

test("remove and health", () => {
  assert.deepEqual(applyEvent([a, b], "service.remove", { id: "a" }), [b]);
  const got = applyEvent([a, b], "health", { id: "b", health: { status: "up", ms: 5 } });
  assert.deepEqual(got[1].health, { status: "up", ms: 5 });
  assert.equal(got[0], a);
});

test("unknown events change nothing", () => {
  const list = [a];
  assert.equal(applyEvent(list, "config", { etag: "x" }), list);
});

test("setOverride puts JSON to the service's path", async () => {
  let call;
  await setOverride("docker:media/app", { pinned: true }, async (url, opts) => {
    call = { url, opts };
    return { ok: true };
  });
  assert.equal(call.url, "./api/overrides/docker:media/app");
  assert.equal(call.opts.method, "PUT");
  assert.equal(call.opts.body, '{"pinned":true}');
  await assert.rejects(setOverride("x", null, async () => ({ ok: false, status: 404 })));
});

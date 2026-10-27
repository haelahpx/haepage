import assert from "node:assert/strict";
import { test } from "node:test";
import { certNotice, expiryRows, whenText } from "./expiry.js";

const now = Date.parse("2026-10-27T12:00:00Z");
const inDays = (d) => new Date(now + d * 86400_000).toISOString();

test("certNotice", () => {
  assert.equal(certNotice(null, now), null);
  assert.equal(certNotice(inDays(60), now), null);
  assert.equal(certNotice(inDays(5.5), now).text, "Cert 5 d");
  assert.equal(certNotice(inDays(0.5), now).text, "Cert ends today");
  assert.equal(certNotice(inDays(-1), now).text, "Cert expired");
  assert.equal(certNotice(inDays(20), now, 30).text, "Cert 20 d");
});

test("whenText", () => {
  assert.equal(whenText(inDays(23.2), now), "in 23 days");
  assert.equal(whenText(inDays(1.5), now), "tomorrow");
  assert.equal(whenText(inDays(0.2), now), "today");
  assert.equal(whenText(inDays(-0.2), now), "ended today");
  assert.equal(whenText(inDays(-1.2), now), "ended 1 day ago");
  assert.equal(whenText(inDays(-3), now), "ended 3 days ago");
});

test("expiryRows sorts soonest first, unknown last", () => {
  const rows = expiryRows(
    [{ name: "NAS", health: { certExpires: inDays(40) } }, { name: "Router", health: { status: "up" } }],
    [{ name: "home.dev", expires: inDays(10) }, { name: "odd.museum", error: "no RDAP" }],
  );
  assert.deepEqual(rows.map((r) => `${r.kind}:${r.name}`), ["domain:home.dev", "cert:NAS", "domain:odd.museum"]);
});

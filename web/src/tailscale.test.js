import assert from "node:assert/strict";
import { test } from "node:test";
import { deviceDetail, mainIp, onlineSummary, osName, seenAgo } from "./tailscale.js";

const now = Date.parse("2026-10-27T12:00:00Z");

test("seenAgo", () => {
  assert.equal(seenAgo(null, now), "offline");
  assert.equal(seenAgo("2026-10-27T11:59:30Z", now), "seen just now");
  assert.equal(seenAgo("2026-10-27T11:15:00Z", now), "seen 45m ago");
  assert.equal(seenAgo("2026-10-27T07:00:00Z", now), "seen 5h ago");
  assert.equal(seenAgo("2026-10-20T12:00:00Z", now), "seen 7d ago");
});

test("mainIp prefers IPv4", () => {
  assert.equal(mainIp(["fd7a:115c::1", "100.64.0.1"]), "100.64.0.1");
  assert.equal(mainIp(["fd7a:115c::1"]), "fd7a:115c::1");
  assert.equal(mainIp([]), "");
});

test("summary and details", () => {
  assert.equal(onlineSummary([{ online: true }, { online: false }, { online: true }]), "2 of 3 online");
  assert.equal(osName("android"), "Android");
  assert.equal(osName("plan9"), "plan9");
  assert.equal(deviceDetail({ os: "linux", ips: ["100.64.0.3"], online: true, connection: "relay" }, now), "Linux · 100.64.0.3 · relayed");
  assert.equal(deviceDetail({ os: "iOS", ips: ["100.64.0.4"], online: true, connection: "direct" }, now), "iOS · 100.64.0.4");
  assert.equal(deviceDetail({ os: "windows", ips: [], online: false, lastSeen: "2026-10-26T12:00:00Z" }, now), "Windows · seen 1d ago");
});

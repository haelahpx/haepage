import assert from "node:assert/strict";
import { test } from "node:test";
import {
  chipFor,
  downloads,
  formatClock,
  formatDay,
  nowPlaying,
  pendingRequests,
  posterFor,
  shouldNotify,
  timeAgo,
  upcoming,
} from "./integration.js";

const jf = { service: "jf", kind: "jellyfin", status: "ok", data: { sessions: [{ id: "s1", itemId: "i1", title: "A" }] } };
const seerr = {
  service: "rq",
  kind: "seerr",
  status: "ok",
  data: { pending: 2, requests: [{ id: 1, title: "Old", createdAt: "2026-10-26T10:00:00Z" }, { id: 2, title: "New", createdAt: "2026-10-27T10:00:00Z" }] },
};
const radarr = {
  service: "rd",
  kind: "radarr",
  status: "ok",
  data: { queue: [{ id: 7, title: "Film" }], upcoming: [{ title: "Later", date: "2026-10-30T00:00:00Z" }], health: [] },
};
const sonarr = {
  service: "sn",
  kind: "sonarr",
  status: "ok",
  data: { queue: [], upcoming: [{ title: "Sooner", date: "2026-10-28T00:00:00Z" }], health: [{ type: "warning", message: "Disk" }] },
};
const states = { jf, rq: seerr, rd: radarr, sn: sonarr, broken: { service: "x", kind: "radarr", status: "error", error: "down" } };

test("chips", () => {
  assert.deepEqual(chipFor(jf), { text: "▶ 1", tone: "live", title: "1 watching" });
  assert.equal(chipFor(seerr).text, "2 pending");
  assert.equal(chipFor(radarr).text, "↓ 1");
  assert.deepEqual(chipFor(sonarr), { text: "!", tone: "warn", title: "Disk" });
  assert.equal(chipFor(states.broken).title, "down");
  assert.equal(chipFor({ kind: "jellyfin", status: "ok", data: { sessions: [] } }), null);
  assert.equal(chipFor(undefined), null);
});

test("merged lists skip broken integrations and keep their service", () => {
  assert.deepEqual(nowPlaying(states).map((s) => s.service), ["jf"]);
  assert.deepEqual(pendingRequests(states).map((r) => r.title), ["New", "Old"]);
  assert.deepEqual(downloads(states).map((d) => [d.title, d.kind]), [["Film", "radarr"]]);
  assert.deepEqual(upcoming(states).map((u) => u.title), ["Sooner", "Later"]);
});

test("formatDay", () => {
  const now = new Date(2026, 9, 27, 15, 0);
  assert.equal(formatDay(new Date(2026, 9, 27, 21, 0), now), "Today");
  assert.equal(formatDay(new Date(2026, 9, 28, 1, 0), now), "Tomorrow");
  assert.equal(formatDay(new Date(2026, 9, 30, 12, 0), now), new Date(2026, 9, 30).toLocaleDateString([], { weekday: "long" }));
});

test("timeAgo and formatClock", () => {
  const now = new Date("2026-10-27T12:00:00Z");
  assert.equal(timeAgo("2026-10-27T11:59:30Z", now), "just now");
  assert.equal(timeAgo("2026-10-27T11:45:00Z", now), "15m ago");
  assert.equal(timeAgo("2026-10-27T09:00:00Z", now), "3h ago");
  assert.equal(timeAgo("2026-10-25T12:00:00Z", now), "2d ago");
  assert.equal(formatClock(23 * 60_000), "23:00");
  assert.equal(formatClock((60 + 5) * 60_000 + 9000), "1:05:09");
});

test("notifications skip playback starts", () => {
  assert.equal(shouldNotify({ type: "request.new" }), true);
  assert.equal(shouldNotify({ type: "playback.start" }), false);
});

test("posterFor goes through Haepage", () => {
  assert.equal(posterFor({ service: "docker:media/jellyfin", itemId: "a1" }), "./api/integrations/image?service=docker%3Amedia%2Fjellyfin&item=a1");
});

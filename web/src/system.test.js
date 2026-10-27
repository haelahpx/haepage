import assert from "node:assert/strict";
import { test } from "node:test";
import { formatBytes, formatRate, formatUptime, mainDisk, percent } from "./system.js";

test("formatBytes", () => {
  assert.equal(formatBytes(0), "0 B");
  assert.equal(formatBytes(-5), "0 B");
  assert.equal(formatBytes(512), "512 B");
  assert.equal(formatBytes(1536), "1.5 KB");
  assert.equal(formatBytes(16 * 1024 ** 3), "16.0 GB");
  assert.equal(formatBytes(250 * 1024 ** 3), "250 GB");
  assert.equal(formatBytes(2 * 1024 ** 4), "2.0 TB");
});

test("formatRate", () => {
  assert.equal(formatRate(1024 * 1024 * 3.2), "3.2 MB/s");
});

test("formatUptime", () => {
  assert.equal(formatUptime(0), "just started");
  assert.equal(formatUptime(30), "just started");
  assert.equal(formatUptime(600), "10m");
  assert.equal(formatUptime(3 * 3600 + 120), "3h 2m");
  assert.equal(formatUptime(2 * 86400 + 5 * 3600), "2d 5h");
});

test("percent", () => {
  assert.equal(percent(1, 4), 25);
  assert.equal(percent(5, 0), 0);
  assert.equal(percent(9, 4), 100);
});

test("mainDisk picks the largest", () => {
  assert.equal(mainDisk([{ path: "/", total: 10 }, { path: "/data", total: 50 }]).path, "/data");
  assert.equal(mainDisk([]), null);
});

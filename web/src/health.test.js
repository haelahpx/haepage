import assert from "node:assert/strict";
import { test } from "node:test";
import { probe, unprobeable } from "./health.js";

test("unprobeable", () => {
  assert.equal(unprobeable("http://192.168.1.10", "http:"), null);
  assert.equal(unprobeable("https://a.example.com", "https:"), null);
  assert.match(unprobeable("http://192.168.1.10", "https:"), /https/);
  assert.equal(unprobeable("ssh://host", "http:"), "not a web URL");
  assert.equal(unprobeable("nope", "http:"), "invalid URL");
});

test("probe reports up on any response and down on errors", async () => {
  assert.equal(await probe("http://x", { fetchFn: async () => ({ type: "opaque" }) }), "up");
  const fail = async () => {
    throw new TypeError("failed");
  };
  assert.equal(await probe("http://x", { fetchFn: fail }), "down");
});

test("probe uses no-cors", async () => {
  let opts;
  await probe("http://x", {
    fetchFn: async (_url, o) => {
      opts = o;
      return {};
    },
  });
  assert.equal(opts.mode, "no-cors");
});

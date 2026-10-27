import assert from "node:assert/strict";
import { test } from "node:test";
import { hostOf, networkOf, resolveLink } from "./links.js";

test("networkOf classifies hostnames", () => {
  const cases = {
    "server.tail1234.ts.net": "tailscale",
    "100.64.0.1": "tailscale",
    "100.127.255.254": "tailscale",
    "[fd7a:115c:a1e0::1]": "tailscale",
    "100.63.0.1": "public",
    "100.128.0.1": "public",
    "192.168.1.10": "lan",
    "10.0.0.5": "lan",
    "172.16.0.1": "lan",
    "172.31.255.255": "lan",
    "172.32.0.1": "public",
    "127.0.0.1": "lan",
    "169.254.10.10": "lan",
    "[::1]": "lan",
    "[fd00::5]": "lan",
    "[fe80::1]": "lan",
    "[2001:db8::1]": "public",
    localhost: "lan",
    server: "lan",
    "nas.local": "lan",
    "nas.lan": "lan",
    "nas.home.arpa": "lan",
    "": "lan",
    "home.example.com": "public",
    "8.8.8.8": "public",
  };
  for (const [host, want] of Object.entries(cases)) {
    assert.equal(networkOf(host), want, host);
  }
});

const urls = {
  lan: "http://192.168.1.10:8096",
  tailscale: "http://server.example.ts.net:8096",
  public: "https://media.example.com",
};

test("each network prefers its own URL", () => {
  assert.equal(resolveLink(urls, "192.168.1.2").kind, "lan");
  assert.equal(resolveLink(urls, "box.example.ts.net").kind, "tailscale");
  assert.equal(resolveLink(urls, "home.example.com").kind, "public");
});

test("fallback order follows the table", () => {
  assert.equal(resolveLink({ public: "p", tailscale: "t" }, "192.168.1.2").kind, "tailscale");
  assert.equal(resolveLink({ public: "p" }, "192.168.1.2").kind, "public");
  assert.equal(resolveLink({ lan: "l", public: "p" }, "100.64.0.1").kind, "lan");
  assert.equal(resolveLink({ lan: "l" }, "100.64.0.1").kind, "lan");
  assert.equal(resolveLink({ lan: "l", tailscale: "t" }, "home.example.com").kind, "tailscale");
  assert.equal(resolveLink({ lan: "l" }, "home.example.com").kind, "lan");
});

test("others lists the remaining URLs in order", () => {
  const got = resolveLink(urls, "192.168.1.2");
  assert.deepEqual(
    got.others.map((o) => o.kind),
    ["tailscale", "public"],
  );
});

test("no URLs gives null", () => {
  assert.equal(resolveLink({}, "localhost"), null);
  assert.equal(resolveLink(undefined, "localhost"), null);
});

test("{host} becomes the visitor's host name", () => {
  const detected = { lan: "http://{host}:8096" };
  assert.equal(resolveLink(detected, "192.168.1.10").url, "http://192.168.1.10:8096");
  assert.equal(resolveLink(detected, "box.tail1234.ts.net").url, "http://box.tail1234.ts.net:8096");
  assert.equal(resolveLink(detected, "[fd00::5]").url, "http://[fd00::5]:8096");
  assert.equal(resolveLink(detected, "").url, "http://localhost:8096");
});

test("hostOf", () => {
  assert.equal(hostOf("http://192.168.1.10:8096/web"), "192.168.1.10:8096");
  assert.equal(hostOf("not a url"), "not a url");
});

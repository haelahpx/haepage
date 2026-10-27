import assert from "node:assert/strict";
import { test } from "node:test";
import { migrate } from "./migrate.js";
import { ConflictError, DRAFT_KEY, clearDraft, loadConfig, loadServerConfig, saveDraft, saveServerConfig } from "./store.js";

function memoryStorage(initial = {}) {
  const data = new Map(Object.entries(initial));
  return {
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => data.set(k, String(v)),
    removeItem: (k) => data.delete(k),
  };
}

const fileReply =
  (body, { status = 200, lastModified } = {}) =>
  async () => ({
    ok: status >= 200 && status < 300,
    status,
    headers: { get: (h) => (h === "Last-Modified" ? (lastModified ?? null) : null) },
    json: async () => (typeof body === "string" ? JSON.parse(body) : body),
  });

test("migrate fills defaults and keeps unknown fields", () => {
  const got = migrate({ version: 1, settings: { title: "Lab" }, futureField: { a: 1 } });
  assert.equal(got.settings.title, "Lab");
  assert.equal(got.settings.theme, "tonal");
  assert.deepEqual(got.services, []);
  assert.deepEqual(got.futureField, { a: 1 });
});

test("migrate renames the old built-in theme ids", () => {
  assert.equal(migrate({ settings: { theme: "android" } }).settings.theme, "tonal");
  assert.equal(migrate({ settings: { theme: "apple" } }).settings.theme, "glass");
  assert.equal(migrate({ settings: { theme: "https://x.example/t.css" } }).settings.theme, "https://x.example/t.css");
});

test("migrate treats a missing version as 1", () => {
  assert.equal(migrate({}).version, 1);
});

test("migrate replaces wrong types with empty defaults", () => {
  const got = migrate({ services: "nope", settings: [], overrides: null });
  assert.deepEqual(got.services, []);
  assert.equal(got.settings.title, "Home");
  assert.deepEqual(got.overrides, {});
});

test("migrate rejects newer versions and non-objects", () => {
  assert.throws(() => migrate({ version: 2 }), /only understands up to version 1/);
  assert.throws(() => migrate([]), /JSON object/);
  assert.throws(() => migrate(null), /JSON object/);
});

test("missing file gives defaults without an error", async () => {
  const got = await loadConfig({ fetchFn: fileReply({}, { status: 404 }), storage: memoryStorage() });
  assert.equal(got.source, "defaults");
  assert.equal(got.error, null);
});

test("file is used when there is no draft", async () => {
  const got = await loadConfig({ fetchFn: fileReply({ settings: { title: "File" } }), storage: memoryStorage() });
  assert.equal(got.source, "file");
  assert.equal(got.config.settings.title, "File");
});

test("invalid JSON reports an error and falls back to defaults", async () => {
  const got = await loadConfig({ fetchFn: fileReply("{oops"), storage: memoryStorage() });
  assert.equal(got.source, "defaults");
  assert.ok(got.error);
});

test("a draft newer than the file wins", async () => {
  const storage = memoryStorage();
  saveDraft(migrate({ settings: { title: "Draft" } }), storage, Date.parse("2026-10-02T00:00:00Z"));
  const got = await loadConfig({
    fetchFn: fileReply({ settings: { title: "File" } }, { lastModified: "Thu, 01 Oct 2026 00:00:00 GMT" }),
    storage,
  });
  assert.equal(got.source, "draft");
  assert.equal(got.config.settings.title, "Draft");
  assert.equal(got.file.settings.title, "File");
});

test("a file newer than the draft wins", async () => {
  const storage = memoryStorage();
  saveDraft(migrate({ settings: { title: "Draft" } }), storage, Date.parse("2026-10-01T00:00:00Z"));
  const got = await loadConfig({
    fetchFn: fileReply({ settings: { title: "File" } }, { lastModified: "Fri, 02 Oct 2026 00:00:00 GMT" }),
    storage,
  });
  assert.equal(got.source, "file");
});

test("a corrupt draft is ignored", async () => {
  const storage = memoryStorage({ [DRAFT_KEY]: "not json" });
  const got = await loadConfig({ fetchFn: fileReply({}, { status: 404 }), storage });
  assert.equal(got.source, "defaults");
});

test("clearDraft removes the draft", () => {
  const storage = memoryStorage();
  saveDraft(migrate({}), storage);
  clearDraft(storage);
  assert.equal(storage.getItem(DRAFT_KEY), null);
});

test("storage failures do not throw", () => {
  const broken = {
    setItem() {
      throw new Error("quota");
    },
    removeItem() {
      throw new Error("blocked");
    },
  };
  assert.equal(saveDraft(migrate({}), broken), false);
  clearDraft(broken);
});

test("loadServerConfig reads the config and ETag", async () => {
  const fetchFn = async (url) => {
    assert.equal(url, "./api/config");
    return { ok: true, headers: { get: () => '"abc"' }, json: async () => ({ settings: { title: "Srv" } }) };
  };
  const got = await loadServerConfig(fetchFn);
  assert.equal(got.etag, '"abc"');
  assert.equal(got.source, "server");
  assert.equal(got.config.settings.title, "Srv");
});

test("saveServerConfig sends If-Match and returns the new ETag", async () => {
  let sent;
  const fetchFn = async (url, opts) => {
    sent = opts;
    return { ok: true, status: 200, headers: { get: () => '"new"' }, json: async () => ({ etag: '"new"' }) };
  };
  assert.equal(await saveServerConfig({ version: 1 }, '"old"', fetchFn), '"new"');
  assert.equal(sent.method, "PUT");
  assert.equal(sent.headers["If-Match"], '"old"');
});

test("saveServerConfig reports conflicts and errors", async () => {
  const reply = (status, body) => async () => ({ ok: false, status, headers: { get: () => null }, json: async () => body });
  await assert.rejects(saveServerConfig({}, '"x"', reply(412, {})), ConflictError);
  await assert.rejects(saveServerConfig({}, '"x"', reply(400, { error: "invalid config" })), /invalid config/);
});

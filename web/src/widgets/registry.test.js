import assert from "node:assert/strict";
import { test } from "node:test";
import { WIDGET_TYPES, newWidget, placeWidgets } from "./registry.js";
import { safeUrl } from "./api.js";

test("placeWidgets", () => {
  const ws = [
    { id: "a", type: "clock" },
    { id: "b", type: "notes", area: "media" },
    { id: "c", type: "todo", area: "sidebar" },
    { id: "d", type: "todo", area: "gone" },
    { id: "e", type: "clock", area: "header" },
    { type: "clock" },
    null,
  ];
  const p = placeWidgets(ws, ["media", "net"]);
  assert.deepEqual(p.top.map((w) => w.id), ["a", "e"]);
  assert.deepEqual(Object.keys(p.groups), ["media"]);
  assert.deepEqual(p.groups.media.map((w) => w.id), ["b"]);
  assert.deepEqual(p.bottom.map((w) => w.id), ["c", "d"]);
  assert.deepEqual(placeWidgets(undefined, []), { top: [], bottom: [], groups: {} });
});

test("newWidget copies defaults and avoids taken ids", () => {
  const seq = [0.1, 0.1, 0.5];
  const w = newWidget("todo", [`todo-${(0.1).toString(36).slice(2, 6)}`], () => seq.shift());
  assert.equal(w.type, "todo");
  assert.equal(w.id, `todo-${(0.5).toString(36).slice(2, 6)}`);
  w.options.items.push(1);
  assert.deepEqual(WIDGET_TYPES.todo.defaults.items, [], "defaults must not be shared");
});

test("every type has a name, defaults and a loader", () => {
  for (const [type, t] of Object.entries(WIDGET_TYPES)) {
    assert.ok(t.name && t.defaults && typeof t.load === "function", type);
  }
});

test("safeUrl allows only http and https", () => {
  assert.equal(safeUrl("https://a.example/x"), "https://a.example/x");
  assert.equal(safeUrl("http://192.168.1.2:8080"), "http://192.168.1.2:8080/");
  for (const bad of ["javascript:alert(1)", "data:text/html,x", "nope", ""]) assert.equal(safeUrl(bad), null, bad);
});

import assert from "node:assert/strict";
import { test } from "node:test";
import { byOrder, canDropService, moveGroup, moveService, moveWidget, withLayout } from "./edit.js";
import { arrange } from "./layout.js";
import { placeWidgets } from "./widgets/registry.js";

const base = () => ({
  groups: [
    { id: "media", name: "Media", order: 0 },
    { id: "net", name: "Network", order: 1 },
  ],
  services: [
    { id: "a", name: "A", group: "media" },
    { id: "b", name: "B", group: "media" },
    { id: "r", name: "Router", group: "net" },
    { id: "loose", name: "Loose" },
  ],
  overrides: { "docker:x": { pinned: true, group: "media" } },
  widgets: [],
});

const live = (config) => [...config.services, { id: "docker:x", name: "X", group: "media", source: "docker" }];
const page = (config) => arrange(config.groups, byOrder(withLayout(live(config), config)));
const ids = (sections, id) => sections.find((g) => g.id === id)?.services.map((s) => s.id);

test("byOrder sorts by order and keeps the rest in place, last", () => {
  const out = byOrder([{ id: "x" }, { id: "y", order: 1 }, { id: "z" }, { id: "w", order: 0 }]);
  assert.deepEqual(out.map((i) => i.id), ["w", "y", "x", "z"]);
});

test("moveService reorders within a group", () => {
  const config = base();
  const next = moveService(config, page(config), "docker:x", "media", 0);
  assert.deepEqual(ids(page(next), "media"), ["docker:x", "a", "b"]);
  assert.equal(next.overrides["docker:x"].order, 0);
  assert.equal(next.services[0].order, 1);
  assert.equal(config.services[0].order, undefined, "the old config is untouched");
});

test("moveService moves a card you added to another group", () => {
  const config = base();
  const next = moveService(config, page(config), "a", "net", 1);
  assert.deepEqual(ids(page(next), "net"), ["r", "a"]);
  assert.deepEqual(ids(page(next), "media"), ["b", "docker:x"]);
  assert.equal(next.services.find((s) => s.id === "a").group, "net");
});

test("moveService pins a detected service in its new group, keeping its rename", () => {
  const config = base();
  config.overrides["docker:x"].name = "Renamed";
  const next = moveService(config, page(config), "docker:x", "net", 0);
  assert.deepEqual(next.overrides["docker:x"], { pinned: true, hidden: false, group: "net", name: "Renamed", order: 0 });
  assert.deepEqual(ids(page(next), "net"), ["docker:x", "r"]);
});

test("Other takes only services you added; New takes none", () => {
  const config = base();
  assert.equal(canDropService(config, "a", ""), true);
  assert.equal(canDropService(config, "docker:x", ""), false);
  assert.equal(canDropService(config, "a", "inbox"), false);
  assert.equal(moveService(config, page(config), "docker:x", "", 0), config);
  const next = moveService(config, page(config), "a", "", 0);
  assert.equal("group" in next.services.find((s) => s.id === "a"), false);
  assert.deepEqual(ids(page(next), ""), ["a", "loose"]);
});

test("withLayout applies the config's group before the server has caught up", () => {
  const config = base();
  const moved = moveService(config, page(config), "docker:x", "net", 0);
  const fromServer = [{ id: "docker:x", group: "media" }];
  assert.equal(withLayout(fromServer, moved)[0].group, "net");
  assert.equal(withLayout([{ id: "docker:y", group: "inbox" }], moved)[0].group, "inbox");
});

test("moveGroup renumbers and defines groups that only services named", () => {
  const config = base();
  config.services.push({ id: "c", name: "C", group: "games" });
  config.groups.push({ id: "empty", name: "Empty", order: 5 });
  const next = moveGroup(config, page(config), "games", 0);
  assert.deepEqual(
    next.groups.map((g) => [g.id, g.order]),
    [
      ["games", 0],
      ["media", 1],
      ["net", 2],
      ["empty", 3],
    ],
  );
  assert.equal(next.groups[0].name, "Games");
  assert.deepEqual(
    page(next).map((g) => g.id),
    ["games", "media", "net", ""],
  );
});

test("moveWidget moves within and between areas", () => {
  const config = {
    ...base(),
    widgets: [
      { id: "w1", type: "clock" },
      { id: "w2", type: "notes", area: "media" },
      { id: "w3", type: "todo", area: "header" },
      { id: "w4", type: "rss", area: "sidebar" },
    ],
  };
  const groupIds = ["media", "net"];
  const lists = (c) => {
    const p = placeWidgets(c.widgets, groupIds);
    return { top: p.top.map((w) => w.id), media: p.groups.media?.map((w) => w.id), bottom: p.bottom.map((w) => w.id) };
  };
  assert.deepEqual(lists(moveWidget(config, groupIds, "w3", "header", 0)).top, ["w3", "w1"]);
  const intoGroup = moveWidget(config, groupIds, "w1", "media", 1);
  assert.deepEqual(lists(intoGroup), { top: ["w3"], media: ["w2", "w1"], bottom: ["w4"] });
  assert.deepEqual(lists(moveWidget(config, groupIds, "w2", "net", 0)).media, undefined);
  assert.deepEqual(lists(moveWidget(config, groupIds, "w1", "sidebar", 0)).bottom, ["w1", "w4"]);
});

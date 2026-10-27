import assert from "node:assert/strict";
import { test } from "node:test";
import { arrange } from "./layout.js";

const s = (id, group) => ({ id, group });

test("inbox first, config groups in order, extra groups, then Other", () => {
  const groups = [
    { id: "net", name: "Network", order: 2 },
    { id: "media", name: "Media", order: 1 },
    { id: "empty", name: "Empty", order: 3 },
  ];
  const services = [s("a", "net"), s("b", "media"), s("c", "inbox"), s("d", "home-lab"), s("e"), s("f", "home-lab")];
  const got = arrange(groups, services);
  assert.deepEqual(
    got.map((g) => [g.id, g.name, g.services.map((x) => x.id).join("")]),
    [
      ["inbox", "New", "c"],
      ["media", "Media", "b"],
      ["net", "Network", "a"],
      ["home-lab", "Home Lab", "df"],
      ["", "Other", "e"],
    ],
  );
  assert.equal(got[0].inbox, true);
});

test("every service appears exactly once", () => {
  const services = [s("a", "x"), s("b", "inbox"), s("c"), s("d", "x")];
  const ids = arrange([], services).flatMap((g) => g.services.map((x) => x.id));
  assert.deepEqual(ids.sort(), ["a", "b", "c", "d"]);
});

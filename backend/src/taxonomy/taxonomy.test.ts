import { test } from "node:test";
import assert from "node:assert/strict";
import {
  TAXONOMY,
  flattenTaxonomy,
  leafTags,
  VALID_CODES,
  FALLBACK_CODE,
} from "./taxonomy.js";

test("flattenTaxonomy includes domains and leaves", () => {
  const all = flattenTaxonomy();
  assert.ok(all.length > leafTags().length, "flattened should include parent domains too");
  assert.ok(all.some((n) => n.code === "DISASTER"));
  assert.ok(all.some((n) => n.code === "DISASTER.EARTHQUAKE"));
});

test("leafTags returns only nodes without children", () => {
  for (const t of leafTags()) assert.equal(t.children, undefined);
});

test("VALID_CODES contains every flattened code", () => {
  for (const n of flattenTaxonomy()) assert.ok(VALID_CODES.has(n.code));
});

test("fallback code is a valid leaf", () => {
  assert.ok(VALID_CODES.has(FALLBACK_CODE));
  assert.ok(leafTags().some((t) => t.code === FALLBACK_CODE));
});

test("every top-level domain has children", () => {
  for (const d of TAXONOMY) assert.ok((d.children?.length ?? 0) > 0);
});

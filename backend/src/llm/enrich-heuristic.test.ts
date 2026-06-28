import "../test-utils/env.js";
import { test } from "node:test";
import assert from "node:assert/strict";
import { enrichArticle } from "./enrich.js";

// No OPENAI_API_KEY in the test env → the gateway returns null and enrichArticle
// must fall back to the deterministic heuristic classifier.

test("heuristic classifies an earthquake article as HIGH severity disaster", async () => {
  const r = await enrichArticle({
    title: "Magnitude 6.8 earthquake hits Mindanao",
    body: "A strong earthquake and tremor struck the region; aftershocks expected.",
  });
  assert.equal(r.source, "heuristic");
  assert.equal(r.severity, "HIGH");
  assert.ok(r.tags.some((t) => t.code === "DISASTER.EARTHQUAKE"));
  assert.ok(r.confidence > 0 && r.confidence <= 1);
});

test("heuristic falls back to GENERAL.OTHER when nothing matches", async () => {
  const r = await enrichArticle({
    title: "A pleasant afternoon stroll",
    body: "Someone walked gently somewhere unremarkable.",
  });
  assert.equal(r.source, "heuristic");
  assert.equal(r.severity, "MEDIUM");
  assert.deepEqual(
    r.tags.map((t) => t.code),
    ["GENERAL.OTHER"],
  );
});

test("heuristic summary falls back to the title when the body is empty", async () => {
  const r = await enrichArticle({ title: "Just a headline", body: "" });
  assert.equal(r.source, "heuristic");
  assert.equal(r.summary, "Just a headline");
});

test("heuristic caps tags at three", async () => {
  const r = await enrichArticle({
    title: "Earthquake, flood, cyclone and wildfire all strike amid war and data breach",
    body: "earthquake flood cyclone wildfire war data breach outbreak inflation",
  });
  assert.ok(r.tags.length <= 3);
});

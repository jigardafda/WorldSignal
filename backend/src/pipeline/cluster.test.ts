import "../test-utils/env.js";
import { test, beforeEach, after } from "node:test";
import assert from "node:assert/strict";
import { prisma } from "../db/prisma.js";
import { resetDb } from "../test-utils/db.js";
import { tokenSetString } from "../lib/text.js";
import { clusterArticle } from "./cluster.js";

let sourceId: string;

beforeEach(async () => {
  await resetDb();
  const s = await prisma.source.create({ data: { name: "S", url: `https://s-${Math.random()}.com` } });
  sourceId = s.id;
});
after(() => prisma.$disconnect());

async function makeArticle(title: string, body: string) {
  return prisma.article.create({
    data: { sourceId, title, body, tokenSet: tokenSetString(`${title} ${body}`) },
  });
}

test("first article creates a new signal", async () => {
  const a = await makeArticle("Earthquake Mindanao", "earthquake mindanao region philippines tsunami damage");
  const res = await clusterArticle(a.id);
  assert.ok(res);
  assert.equal(res!.isNew, true);
  const sig = await prisma.signal.findUnique({ where: { id: res!.signalId } });
  assert.equal(sig?.sourceCount, 1);
});

test("a similar article joins the existing signal", async () => {
  const a = await makeArticle("Earthquake Mindanao", "earthquake mindanao region philippines tsunami damage reported");
  const first = await clusterArticle(a.id);
  const b = await makeArticle("Earthquake Mindanao", "earthquake mindanao region philippines tsunami damage assessment");
  const second = await clusterArticle(b.id);
  assert.equal(second!.isNew, false);
  assert.equal(second!.signalId, first!.signalId);
  const sig = await prisma.signal.findUnique({ where: { id: first!.signalId } });
  assert.equal(sig?.sourceCount, 2);
});

test("a dissimilar article creates a separate signal", async () => {
  const a = await makeArticle("Earthquake Mindanao", "earthquake mindanao region philippines tsunami");
  const first = await clusterArticle(a.id);
  const b = await makeArticle("Stock market rallies", "stock market shares nasdaq investors rally gains");
  const second = await clusterArticle(b.id);
  assert.notEqual(second!.signalId, first!.signalId);
  assert.equal(second!.isNew, true);
});

test("clustering an already-linked article is idempotent", async () => {
  const a = await makeArticle("Earthquake Mindanao", "earthquake mindanao region philippines tsunami");
  const first = await clusterArticle(a.id);
  const again = await clusterArticle(a.id);
  assert.equal(again!.signalId, first!.signalId);
  assert.equal(again!.isNew, false);
});

test("handles a candidate signal without tokenSet metadata and a null-tokenSet article", async () => {
  const now = new Date();
  // bare signal created outside the pipeline → no metadata.tokenSet
  await prisma.signal.create({
    data: { title: "Bare", summary: "s", firstSeenAt: now, lastSeenAt: now },
  });
  const a = await prisma.article.create({
    data: { sourceId, title: "Unrelated headline", body: "totally different", tokenSet: null },
  });
  const res = await clusterArticle(a.id);
  assert.ok(res);
  assert.equal(res!.isNew, true); // no similarity → new signal
});

test("uses the article publish date as firstSeenAt when present", async () => {
  const when = new Date("2026-06-10T00:00:00Z");
  const a = await prisma.article.create({
    data: {
      sourceId,
      title: "Dated story",
      body: "dated body content here",
      tokenSet: tokenSetString("dated story dated body content here"),
      publishedAt: when,
    },
  });
  const res = await clusterArticle(a.id);
  const sig = await prisma.signal.findUnique({ where: { id: res!.signalId } });
  assert.equal(sig?.firstSeenAt.toISOString(), when.toISOString());
});

test("returns null for a missing article", async () => {
  assert.equal(await clusterArticle("nope"), null);
});

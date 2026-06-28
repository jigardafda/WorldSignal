import "../test-utils/env.js";
import { test, beforeEach, after } from "node:test";
import assert from "node:assert/strict";
import { prisma } from "../db/prisma.js";
import { resetDb } from "../test-utils/db.js";
import { normalizeRawItem } from "./normalize.js";

async function makeSource() {
  return prisma.source.create({ data: { name: "S", url: `https://s-${Math.random()}.com` } });
}
async function makeRaw(sourceId: string, over: Record<string, unknown> = {}) {
  return prisma.rawItem.create({
    data: {
      sourceId,
      rawTitle: "Quake hits region",
      rawContent: "A magnitude earthquake struck the area today.",
      rawUrl: "https://news.com/quake?utm_source=tw&id=9",
      ...over,
    },
  });
}

beforeEach(resetDb);
after(() => prisma.$disconnect());

test("normalizes a raw item into an article and canonicalizes the url", async () => {
  const src = await makeSource();
  const raw = await makeRaw(src.id);
  const articleId = await normalizeRawItem(raw.id);
  assert.ok(articleId);
  const article = await prisma.article.findUnique({ where: { id: articleId! } });
  assert.equal(article?.canonicalUrl, "https://news.com/quake?id=9");
  assert.ok(article?.contentHash);
  assert.ok(article?.tokenSet && article.tokenSet.length > 0);
  const updated = await prisma.rawItem.findUnique({ where: { id: raw.id } });
  assert.equal(updated?.status, "PARSED");
});

test("exact duplicate by content hash is skipped", async () => {
  const src = await makeSource();
  const r1 = await makeRaw(src.id, { sourceGuid: "g1", rawUrl: "https://a.com/1" });
  await normalizeRawItem(r1.id);
  const r2 = await makeRaw(src.id, { sourceGuid: "g2", rawUrl: "https://b.com/2" });
  const res = await normalizeRawItem(r2.id);
  assert.equal(res, null);
  const updated = await prisma.rawItem.findUnique({ where: { id: r2.id } });
  assert.equal(updated?.status, "DUPLICATE");
});

test("duplicate by canonical url is skipped", async () => {
  const src = await makeSource();
  const r1 = await makeRaw(src.id, { sourceGuid: "x1" });
  await normalizeRawItem(r1.id);
  const r2 = await makeRaw(src.id, {
    sourceGuid: "x2",
    rawTitle: "Totally different headline words",
    rawContent: "Different body entirely unrelated content.",
  });
  const res = await normalizeRawItem(r2.id);
  assert.equal(res, null); // same canonical url
});

test("missing title marks the raw item FAILED", async () => {
  const src = await makeSource();
  const raw = await makeRaw(src.id, { rawTitle: "" });
  assert.equal(await normalizeRawItem(raw.id), null);
  const updated = await prisma.rawItem.findUnique({ where: { id: raw.id } });
  assert.equal(updated?.status, "FAILED");
});

test("re-normalizing an already-parsed item returns the existing article", async () => {
  const src = await makeSource();
  const raw = await makeRaw(src.id);
  const first = await normalizeRawItem(raw.id);
  const second = await normalizeRawItem(raw.id);
  assert.equal(second, first);
});

test("handles a raw item with no body and no url", async () => {
  const src = await makeSource();
  const raw = await prisma.rawItem.create({
    data: { sourceId: src.id, rawTitle: "Headline only", rawContent: null, rawUrl: null },
  });
  const articleId = await normalizeRawItem(raw.id);
  assert.ok(articleId);
  const article = await prisma.article.findUnique({ where: { id: articleId! } });
  assert.equal(article?.canonicalUrl, null);
  assert.equal(article?.summary, null); // empty body → null summary
  assert.equal(article?.body, "");
});

test("re-normalizing a DUPLICATE raw item returns null", async () => {
  const src = await makeSource();
  const r1 = await makeRaw(src.id, { sourceGuid: "d1", rawUrl: "https://dd.com/1" });
  await normalizeRawItem(r1.id);
  const r2 = await makeRaw(src.id, { sourceGuid: "d2", rawUrl: "https://dd.com/2" });
  await normalizeRawItem(r2.id); // marks r2 DUPLICATE (same content hash)
  assert.equal(await normalizeRawItem(r2.id), null); // re-run hits the DUPLICATE branch
});

test("returns null for a missing raw item id", async () => {
  assert.equal(await normalizeRawItem("nonexistent"), null);
});

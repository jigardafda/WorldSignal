import "../test-utils/env.js";
import { test, beforeEach, after } from "node:test";
import assert from "node:assert/strict";
import { prisma } from "../db/prisma.js";
import { resetDb, seedTaxonomy } from "../test-utils/db.js";
import { enrichSignal } from "./enrichSignal.js";

beforeEach(async () => {
  await resetDb();
  await seedTaxonomy();
});
after(() => prisma.$disconnect());

async function makeSource(credibility = 0.5) {
  return prisma.source.create({
    data: { name: "S", url: `https://s-${Math.random()}.com`, credibility },
  });
}

async function signalWithArticles(bodies: { title: string; body: string; cred?: number }[]) {
  const now = new Date();
  const signal = await prisma.signal.create({
    data: { title: "tmp", summary: "tmp", firstSeenAt: now, lastSeenAt: now, sourceCount: bodies.length },
  });
  let i = 0;
  for (const b of bodies) {
    const src = await makeSource(b.cred ?? 0.5);
    const article = await prisma.article.create({
      data: { sourceId: src.id, title: b.title, body: b.body },
    });
    await prisma.signalArticle.create({
      data: { signalId: signal.id, articleId: article.id, relationType: i === 0 ? "PRIMARY" : "SUPPORTING" },
    });
    i++;
  }
  return signal.id;
}

test("enriches a single-source signal and stays UNVERIFIED", async () => {
  const id = await signalWithArticles([
    { title: "Magnitude 6.8 earthquake hits Mindanao", body: "earthquake magnitude tremor struck region" },
  ]);
  await enrichSignal(id);
  const s = await prisma.signal.findUnique({ where: { id }, include: { tags: { include: { tag: true } } } });
  assert.equal(s?.status, "UNVERIFIED");
  assert.equal(s?.severity, "HIGH");
  assert.ok(s!.confidence > 0 && s!.confidence <= 1);
  assert.ok(s?.publishedAt);
  assert.ok(s!.tags.some((t) => t.tag.code === "DISASTER.EARTHQUAKE"));
  assert.equal((s!.metadata as { enrichmentSource: string }).enrichmentSource, "heuristic");
});

test("three independent sources mark the signal CONFIRMED", async () => {
  const id = await signalWithArticles([
    { title: "Quake hits", body: "earthquake magnitude region", cred: 0.9 },
    { title: "Quake reported", body: "earthquake magnitude struck", cred: 0.8 },
    { title: "Strong quake", body: "earthquake tremor aftershock", cred: 0.95 },
  ]);
  await enrichSignal(id);
  const s = await prisma.signal.findUnique({ where: { id } });
  assert.equal(s?.status, "CONFIRMED");
});

test("two sources mark the signal DEVELOPING", async () => {
  const id = await signalWithArticles([
    { title: "Quake hits", body: "earthquake magnitude region" },
    { title: "Quake reported", body: "earthquake magnitude struck" },
  ]);
  await enrichSignal(id);
  const s = await prisma.signal.findUnique({ where: { id } });
  assert.equal(s?.status, "DEVELOPING");
});

test("picks the longest-body article when there is no PRIMARY link", async () => {
  const now = new Date();
  const signal = await prisma.signal.create({
    data: { title: "tmp", summary: "tmp", firstSeenAt: now, lastSeenAt: now },
  });
  const src = await makeSource();
  // one article with no body, one with a longer body — both SUPPORTING (no PRIMARY)
  const short = await prisma.article.create({ data: { sourceId: src.id, title: "short", body: null } });
  const long = await prisma.article.create({
    data: { sourceId: src.id, title: "long", body: "earthquake magnitude tremor struck the region hard" },
  });
  await prisma.signalArticle.create({ data: { signalId: signal.id, articleId: short.id, relationType: "SUPPORTING" } });
  await prisma.signalArticle.create({ data: { signalId: signal.id, articleId: long.id, relationType: "SUPPORTING" } });
  await enrichSignal(signal.id);
  const s = await prisma.signal.findUnique({ where: { id: signal.id }, include: { tags: { include: { tag: true } } } });
  assert.ok(s!.tags.some((t) => t.tag.code === "DISASTER.EARTHQUAKE"));
});

test("no-op for a signal without articles or a missing signal", async () => {
  const now = new Date();
  const empty = await prisma.signal.create({
    data: { title: "x", summary: "x", firstSeenAt: now, lastSeenAt: now },
  });
  await enrichSignal(empty.id); // should not throw
  await enrichSignal("missing"); // should not throw
  const s = await prisma.signal.findUnique({ where: { id: empty.id } });
  assert.equal(s?.title, "x");
});

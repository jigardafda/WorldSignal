import "../test-utils/env.js";
import { test, before, beforeEach, after, mock } from "node:test";
import assert from "node:assert/strict";
import Fastify, { type FastifyInstance } from "fastify";
import { prisma } from "../db/prisma.js";
import { resetDb, seedTaxonomy } from "../test-utils/db.js";

const sent: string[] = [];
let app: FastifyInstance;

before(async () => {
  mock.module("../jobs/boss.js", {
    namedExports: {
      getBoss: async () => ({ send: async (name: string) => sent.push(name) }),
      stopBoss: async () => {},
    },
  });
  const { registerRoutes } = await import("./routes.js");
  app = Fastify();
  await registerRoutes(app);
  await app.ready();
});

beforeEach(async () => {
  await resetDb();
  await seedTaxonomy();
});

after(async () => {
  await app.close();
  await prisma.$disconnect();
});

async function seedSignal() {
  const now = new Date();
  const src = await prisma.source.create({ data: { name: "Pub", url: `https://p-${Math.random()}.com` } });
  const tag = await prisma.taxonomyTag.findUnique({ where: { code: "DISASTER.EARTHQUAKE" } });
  const article = await prisma.article.create({ data: { sourceId: src.id, title: "Q", canonicalUrl: "https://p.com/q" } });
  const signal = await prisma.signal.create({
    data: { title: "Quake", summary: "s", status: "CONFIRMED", confidence: 0.8, country: "PH", firstSeenAt: now, lastSeenAt: now },
  });
  await prisma.signalArticle.create({ data: { signalId: signal.id, articleId: article.id } });
  await prisma.signalTag.create({ data: { signalId: signal.id, tagId: tag!.id } });
  return signal.id;
}

test("health and CORS preflight", async () => {
  assert.equal((await app.inject({ method: "GET", url: "/health" })).statusCode, 200);
  assert.equal((await app.inject({ method: "OPTIONS", url: "/v1/signals" })).statusCode, 204);
});

test("stats and taxonomy", async () => {
  assert.equal((await app.inject({ url: "/v1/stats" })).statusCode, 200);
  const tax = await app.inject({ url: "/v1/taxonomy" });
  assert.ok(Array.isArray(tax.json()));
});

test("signals listing with all filter branches", async () => {
  await seedSignal();
  const res = await app.inject({
    url: "/v1/signals?search=quake&country=PH&status=CONFIRMED&minConfidence=0.5&tags=DISASTER.EARTHQUAKE&since=2020-01-01&limit=10&offset=0",
  });
  assert.equal(res.statusCode, 200);
  assert.equal(res.json().data.length, 1);

  // no query params at all → exercises the limit/offset defaults
  const bare = await app.inject({ url: "/v1/signals" });
  assert.equal(bare.statusCode, 200);
});

test("signal by id: found and 404", async () => {
  const id = await seedSignal();
  assert.equal((await app.inject({ url: `/v1/signals/${id}` })).statusCode, 200);
  assert.equal((await app.inject({ url: "/v1/signals/nope" })).statusCode, 404);
});

test("source CRUD: create, validation, duplicate, patch, fetch", async () => {
  sent.length = 0;
  const created = await app.inject({
    method: "POST",
    url: "/v1/sources",
    payload: { name: "New", url: "https://new.example.com" },
  });
  assert.equal(created.statusCode, 201);
  assert.equal(sent.length, 1); // enqueued immediate fetch
  const id = created.json().id;

  assert.equal(
    (await app.inject({ method: "POST", url: "/v1/sources", payload: { name: "x" } })).statusCode,
    400,
  );
  assert.equal(
    (await app.inject({ method: "POST", url: "/v1/sources", payload: { name: "Dup", url: "https://new.example.com" } })).statusCode,
    409,
  );

  const patched = await app.inject({ method: "PATCH", url: `/v1/sources/${id}`, payload: { enabled: false } });
  assert.equal(patched.json().enabled, false);

  const fetched = await app.inject({ method: "POST", url: `/v1/sources/${id}/fetch` });
  assert.equal(fetched.json().queued, true);

  assert.equal((await app.inject({ url: "/v1/sources" })).json().data.length, 1);
});

test("subscriptions: list, create, validation; and deliveries listing", async () => {
  const created = await app.inject({
    method: "POST",
    url: "/v1/subscriptions",
    payload: { name: "Sub", channel: "WEBHOOK", filter: { tags: ["TECHNOLOGY.AI"] }, config: { url: "https://hook" } },
  });
  assert.equal(created.statusCode, 201);
  // minimal payload → channel/filter/config defaults applied
  const minimal = await app.inject({ method: "POST", url: "/v1/subscriptions", payload: { name: "Min" } });
  assert.equal(minimal.statusCode, 201);
  assert.equal(
    (await app.inject({ method: "POST", url: "/v1/subscriptions", payload: {} })).statusCode,
    400,
  );
  assert.equal((await app.inject({ url: "/v1/subscriptions" })).json().data.length, 2);
  assert.equal((await app.inject({ url: "/v1/deliveries" })).statusCode, 200);
});

import "../test-utils/env.js";
import { test, before, after, mock } from "node:test";
import assert from "node:assert/strict";
import { prisma } from "../db/prisma.js";
import { resetDb, seedTaxonomy } from "../test-utils/db.js";
import { QUEUES } from "./queues.js";
import { startTestServer, SAMPLE_RSS, type TestServer } from "../test-utils/httpServer.js";

// Fake pg-boss: capture registered handlers and sent jobs so we can drive the
// whole pipeline deterministically without a running queue.
const sent: { name: string; data: any }[] = [];
const handlers = new Map<string, (jobs: any[]) => Promise<void>>();
const fakeBoss = {
  send: async (name: string, data: any) => {
    sent.push({ name, data });
    return "job-id";
  },
  work: async (name: string, handler: (jobs: any[]) => Promise<void>) => {
    handlers.set(name, handler);
  },
  createQueue: async () => {},
  start: async () => {},
};

let workers: typeof import("./workers.js");

before(async () => {
  mock.module("./boss.js", {
    namedExports: { getBoss: async () => fakeBoss, stopBoss: async () => {} },
  });
  workers = await import("./workers.js");
  await workers.registerWorkers();
});

after(() => prisma.$disconnect());

async function run(name: string, data: any, retryCount = 0) {
  const handler = handlers.get(name);
  assert.ok(handler, `no handler for ${name}`);
  await handler!([{ data, retryCount }]);
}

test("enqueue helpers push onto the queue", async () => {
  sent.length = 0;
  await workers.enqueueFetchSource("abc");
  assert.deepEqual(sent.at(-1), { name: QUEUES.fetchSource, data: { sourceId: "abc" } });
});

test("end-to-end: fetch → process → enrich → match → deliver produces a SENT delivery", async () => {
  let server: TestServer | null = null;
  try {
    await resetDb();
    await seedTaxonomy();
    server = await startTestServer(() => ({ body: SAMPLE_RSS }));
    const src = await prisma.source.create({ data: { name: "Feed", url: server.url } });
    const subscriber = await prisma.subscriber.create({ data: { id: "__default__", name: "D" } });
    await prisma.subscription.create({
      data: { subscriberId: subscriber.id, name: "all", channel: "POLLING", filter: {}, config: {} },
    });

    sent.length = 0;
    await run(QUEUES.fetchSource, { sourceId: src.id });

    const processJobs = sent.filter((s) => s.name === QUEUES.processArticle);
    assert.ok(processJobs.length >= 1);
    sent.length = 0;
    for (const j of processJobs) await run(QUEUES.processArticle, j.data);

    const enrichJobs = sent.filter((s) => s.name === QUEUES.enrichSignal);
    assert.ok(enrichJobs.length >= 1);
    sent.length = 0;
    for (const j of enrichJobs) await run(QUEUES.enrichSignal, j.data);

    const matchJobs = sent.filter((s) => s.name === QUEUES.matchSignal);
    sent.length = 0;
    for (const j of matchJobs) await run(QUEUES.matchSignal, j.data);

    const deliverJobs = sent.filter((s) => s.name === QUEUES.sendDelivery);
    assert.ok(deliverJobs.length >= 1);
    for (const j of deliverJobs) await run(QUEUES.sendDelivery, j.data, 0);

    assert.ok((await prisma.signal.count()) >= 1);
    assert.ok((await prisma.deliveryEvent.count({ where: { status: "SENT" } })) >= 1);
  } finally {
    await server?.close();
  }
});

test("sendDelivery handler treats a high retryCount as the final attempt", async () => {
  await resetDb();
  const now = new Date();
  const subscriber = await prisma.subscriber.create({ data: { name: "D" } });
  const sub = await prisma.subscription.create({
    data: { subscriberId: subscriber.id, name: "p", channel: "POLLING", filter: {}, config: {} },
  });
  const signal = await prisma.signal.create({
    data: { title: "t", summary: "s", firstSeenAt: now, lastSeenAt: now },
  });
  const delivery = await prisma.deliveryEvent.create({
    data: { subscriptionId: sub.id, signalId: signal.id, channel: "POLLING", payload: {}, status: "PENDING" },
  });
  await run(QUEUES.sendDelivery, { deliveryId: delivery.id }, 5); // attempt >= retry limit
  const after = await prisma.deliveryEvent.findUnique({ where: { id: delivery.id } });
  assert.equal(after?.status, "SENT");
});

test("processArticle handler ignores exact-duplicate raw items", async () => {
  await resetDb();
  const src = await prisma.source.create({ data: { name: "S", url: "https://dup.example.com" } });
  // raw item with empty title → normalize returns null → handler returns early
  const raw = await prisma.rawItem.create({ data: { sourceId: src.id, rawTitle: "" } });
  sent.length = 0;
  await run(QUEUES.processArticle, { rawItemId: raw.id });
  assert.equal(sent.length, 0);
});

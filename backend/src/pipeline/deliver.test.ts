import "../test-utils/env.js";
import { test, beforeEach, after } from "node:test";
import assert from "node:assert/strict";
import { prisma } from "../db/prisma.js";
import { resetDb, seedTaxonomy } from "../test-utils/db.js";
import { matchSubscriptions, sendDelivery, signPayload } from "./deliver.js";
import { startTestServer, type TestServer } from "../test-utils/httpServer.js";

beforeEach(async () => {
  await resetDb();
  await seedTaxonomy();
});
after(() => prisma.$disconnect());

async function makeSignal(over: Record<string, unknown> = {}, tagCodes: string[] = []) {
  const now = new Date();
  const signal = await prisma.signal.create({
    data: {
      title: "Quake",
      summary: "A quake",
      severity: "HIGH",
      confidence: 0.8,
      country: "PH",
      firstSeenAt: now,
      lastSeenAt: now,
      sourceCount: 2,
      ...over,
    },
  });
  for (const code of tagCodes) {
    const tag = await prisma.taxonomyTag.findUnique({ where: { code } });
    await prisma.signalTag.create({ data: { signalId: signal.id, tagId: tag!.id, confidence: 0.9 } });
  }
  return signal.id;
}

async function makeSub(filter: unknown, config: unknown = {}, channel = "WEBHOOK") {
  const subscriber = await prisma.subscriber.upsert({
    where: { id: "__default__" },
    update: {},
    create: { id: "__default__", name: "Default" },
  });
  return prisma.subscription.create({
    data: { subscriberId: subscriber.id, name: "sub", channel: channel as any, filter: filter as any, config: config as any },
  });
}

test("signPayload is deterministic and sha256-prefixed", () => {
  const a = signPayload("hello");
  assert.ok(a.startsWith("sha256="));
  assert.equal(a, signPayload("hello"));
});

test("matches subscriptions by tag prefix, country, confidence and severity", async () => {
  const id = await makeSignal({}, ["DISASTER.EARTHQUAKE"]);
  await makeSub({}); // empty filter matches everything
  await makeSub({ tags: ["DISASTER"] }); // domain prefix matches the leaf
  await makeSub({ tags: ["ECONOMY"] }); // no match
  await makeSub({ countries: ["US"] }); // wrong country
  await makeSub({ minConfidence: 0.95 }); // too high
  await makeSub({ minSeverity: "CRITICAL" }); // too high

  const ids = await matchSubscriptions(id);
  assert.equal(ids.length, 2);
});

test("matchSubscriptions is idempotent (unique per signal+subscription)", async () => {
  const id = await makeSignal({}, ["DISASTER.EARTHQUAKE"]);
  await makeSub({});
  const first = await matchSubscriptions(id);
  const second = await matchSubscriptions(id);
  assert.equal(first.length, 1);
  assert.equal(second.length, 0);
});

test("a country filter excludes a signal that has no country", async () => {
  const id = await makeSignal({ country: null }, ["DISASTER.EARTHQUAKE"]);
  await makeSub({ countries: ["US"] });
  assert.equal((await matchSubscriptions(id)).length, 0);
});

test("matchSubscriptions returns empty for a missing signal", async () => {
  assert.deepEqual(await matchSubscriptions("missing"), []);
});

async function makeDelivery(channel: string, config: unknown, signalId: string) {
  const sub = await makeSub({}, config, channel);
  return prisma.deliveryEvent.create({
    data: {
      subscriptionId: sub.id,
      signalId,
      channel: channel as any,
      payload: { event_id: "evt_1", data: {} },
      status: "PENDING",
    },
  });
}

test("POLLING delivery is marked SENT without an HTTP call", async () => {
  const id = await makeSignal();
  const d = await makeDelivery("POLLING", {}, id);
  await sendDelivery(d.id, false);
  const after = await prisma.deliveryEvent.findUnique({ where: { id: d.id } });
  assert.equal(after?.status, "SENT");
});

test("webhook delivery posts a signed payload and marks SENT", async () => {
  let server: TestServer | null = null;
  try {
    server = await startTestServer(() => ({ body: "" }));
    const id = await makeSignal();
    const d = await makeDelivery("WEBHOOK", { url: server.url }, id);
    await sendDelivery(d.id, false);
    const after = await prisma.deliveryEvent.findUnique({ where: { id: d.id } });
    assert.equal(after?.status, "SENT");
    assert.equal(server.requests.length, 1);
    assert.ok(String(server.requests[0].headers["x-worldsignal-signature"]).startsWith("sha256="));
  } finally {
    await server?.close();
  }
});

test("webhook with no url is marked FAILED", async () => {
  const id = await makeSignal();
  const d = await makeDelivery("WEBHOOK", {}, id);
  await sendDelivery(d.id, false);
  const after = await prisma.deliveryEvent.findUnique({ where: { id: d.id } });
  assert.equal(after?.status, "FAILED");
});

test("failing webhook throws and is RETRYING when not final", async () => {
  let server: TestServer | null = null;
  try {
    server = await startTestServer(() => ({ body: "" }));
    server.setPostStatus(500);
    const id = await makeSignal();
    const d = await makeDelivery("WEBHOOK", { url: server.url }, id);
    await assert.rejects(() => sendDelivery(d.id, false));
    const after = await prisma.deliveryEvent.findUnique({ where: { id: d.id } });
    assert.equal(after?.status, "RETRYING");
  } finally {
    await server?.close();
  }
});

test("failing webhook on final attempt is DEAD_LETTERED and does not throw", async () => {
  let server: TestServer | null = null;
  try {
    server = await startTestServer(() => ({ body: "" }));
    server.setPostStatus(500);
    const id = await makeSignal();
    const d = await makeDelivery("WEBHOOK", { url: server.url }, id);
    await sendDelivery(d.id, true); // final attempt
    const after = await prisma.deliveryEvent.findUnique({ where: { id: d.id } });
    assert.equal(after?.status, "DEAD_LETTERED");
  } finally {
    await server?.close();
  }
});

test("already-sent delivery and missing delivery are no-ops", async () => {
  const id = await makeSignal();
  const d = await makeDelivery("POLLING", {}, id);
  await sendDelivery(d.id, false); // SENT
  await sendDelivery(d.id, false); // early return, still SENT
  await sendDelivery("missing", false); // no throw
  const after = await prisma.deliveryEvent.findUnique({ where: { id: d.id } });
  assert.equal(after?.attempts, 1);
});

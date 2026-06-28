import "../test-utils/env.js";
import { test, before, beforeEach, after, mock } from "node:test";
import assert from "node:assert/strict";
import { prisma } from "../db/prisma.js";
import { resetDb, seedTaxonomy } from "../test-utils/db.js";

// Mock pg-boss so triggerFetch does not need a running queue.
const sent: { name: string }[] = [];
let yoga: typeof import("./graphql.js")["yoga"];

before(async () => {
  mock.module("../jobs/boss.js", {
    namedExports: {
      getBoss: async () => ({ send: async (name: string) => sent.push({ name }) }),
      stopBoss: async () => {},
    },
  });
  ({ yoga } = await import("./graphql.js"));
});

beforeEach(async () => {
  await resetDb();
  await seedTaxonomy();
});
after(() => prisma.$disconnect());

async function gql(query: string, variables?: Record<string, unknown>) {
  const res = await yoga.fetch("http://localhost/graphql", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ query, variables }),
  });
  const json = (await res.json()) as { data?: any; errors?: { message: string }[] };
  if (json.errors) throw new Error(json.errors[0].message);
  return json.data;
}

async function seedSignal() {
  const now = new Date();
  const src = await prisma.source.create({ data: { name: "Pub", url: `https://p-${Math.random()}.com` } });
  const article = await prisma.article.create({
    data: { sourceId: src.id, title: "Quake", canonicalUrl: "https://p.com/q" },
  });
  const tag = await prisma.taxonomyTag.findUnique({ where: { code: "DISASTER.EARTHQUAKE" } });
  const signal = await prisma.signal.create({
    data: {
      title: "Magnitude 6.8 earthquake",
      summary: "A quake struck",
      status: "CONFIRMED",
      severity: "HIGH",
      confidence: 0.82,
      country: "PH",
      firstSeenAt: now,
      lastSeenAt: now,
    },
  });
  await prisma.signalArticle.create({ data: { signalId: signal.id, articleId: article.id } });
  await prisma.signalTag.create({ data: { signalId: signal.id, tagId: tag!.id, confidence: 0.9 } });
  return signal.id;
}

test("stats and taxonomy queries", async () => {
  const d = await gql(`{ stats taxonomy }`);
  assert.ok(typeof d.stats.signals === "number");
  assert.ok(Array.isArray(d.taxonomy));
});

test("signals query returns tags and sources", async () => {
  const id = await seedSignal();
  const d = await gql(`{ signals { id title tags { code confidence } sources { publisher url } } }`);
  assert.equal(d.signals.length, 1);
  assert.equal(d.signals[0].id, id);
  assert.equal(d.signals[0].tags[0].code, "DISASTER.EARTHQUAKE");
  assert.equal(d.signals[0].sources[0].publisher, "Pub");
});

test("signals filter branches: search, tags, country, status, minConfidence", async () => {
  await seedSignal();
  const q = `query($f: SignalFilter){ signals(filter: $f){ id } }`;
  assert.equal((await gql(q, { f: { search: "earthquake" } })).signals.length, 1);
  assert.equal((await gql(q, { f: { tags: ["DISASTER.EARTHQUAKE"] } })).signals.length, 1);
  assert.equal((await gql(q, { f: { country: "PH" } })).signals.length, 1);
  assert.equal((await gql(q, { f: { status: "CONFIRMED" } })).signals.length, 1);
  assert.equal((await gql(q, { f: { minConfidence: 0.99 } })).signals.length, 0);
});

test("signals query honours explicit limit and offset", async () => {
  await seedSignal();
  const d = await gql(`query($l: Int, $o: Int){ signals(limit: $l, offset: $o){ id } }`, { l: 5, o: 0 });
  assert.equal(d.signals.length, 1);
});

test("createSource accepts all optional fields", async () => {
  const d = await gql(`mutation($i: CreateSourceInput!){ createSource(input: $i){ id } }`, {
    i: {
      name: "Full",
      url: "https://full.example.com",
      type: "ATOM",
      country: "US",
      priority: 0,
      crawlFrequency: 120,
      credibility: 0.9,
    },
  });
  assert.ok(d.createSource.id);
});

test("createSubscription with explicit filter, and a minimal one using defaults", async () => {
  const full = await gql(
    `mutation($i: CreateSubscriptionInput!){ createSubscription(input: $i){ channel filter } }`,
    { i: { name: "Full", channel: "WEBHOOK", filter: { tags: ["TECHNOLOGY.AI"] }, config: { url: "h" } } },
  );
  assert.deepEqual(full.createSubscription.filter, { tags: ["TECHNOLOGY.AI"] });
  const minimal = await gql(
    `mutation($i: CreateSubscriptionInput!){ createSubscription(input: $i){ channel } }`,
    { i: { name: "Minimal" } },
  );
  assert.equal(minimal.createSubscription.channel, "WEBHOOK"); // default applied
});

test("signal(id) returns one and null for missing", async () => {
  const id = await seedSignal();
  assert.equal((await gql(`query($id: ID!){ signal(id: $id){ id } }`, { id })).signal.id, id);
  assert.equal((await gql(`query($id: ID!){ signal(id: $id){ id } }`, { id: "nope" })).signal, null);
});

test("createSource, setSourceEnabled and triggerFetch mutations", async () => {
  const created = await gql(
    `mutation($i: CreateSourceInput!){ createSource(input: $i){ id name } }`,
    { i: { name: "New", url: "https://new.example.com" } },
  );
  assert.equal(created.createSource.name, "New");

  const toggled = await gql(`mutation($id: ID!){ setSourceEnabled(id: $id, enabled: false){ enabled } }`, {
    id: created.createSource.id,
  });
  assert.equal(toggled.setSourceEnabled.enabled, false);

  sent.length = 0;
  const fetched = await gql(`mutation($id: ID!){ triggerFetch(id: $id) }`, { id: created.createSource.id });
  assert.equal(fetched.triggerFetch, true);
  assert.equal(sent.length, 1);
});

test("createSubscription and subscriptions query", async () => {
  const created = await gql(
    `mutation($i: CreateSubscriptionInput!){ createSubscription(input: $i){ id name channel } }`,
    { i: { name: "Sub", channel: "WEBHOOK", config: { url: "https://hook.example.com" } } },
  );
  assert.equal(created.createSubscription.name, "Sub");
  const list = await gql(`{ subscriptions { id name } sources { id } }`);
  assert.equal(list.subscriptions.length, 1);
});

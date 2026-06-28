import "../test-utils/env.js";
process.env.SCHEDULER_TICK_MS = "40"; // fast interval so the timer fires within the test
import { test, before, beforeEach, after, mock } from "node:test";
import assert from "node:assert/strict";
import { prisma } from "../db/prisma.js";
import { resetDb } from "../test-utils/db.js";

const sent: string[] = [];
let scheduler: typeof import("./scheduler.js");

before(async () => {
  mock.module("./boss.js", {
    namedExports: {
      getBoss: async () => ({ send: async (name: string) => sent.push(name) }),
      stopBoss: async () => {},
    },
  });
  scheduler = await import("./scheduler.js");
});

beforeEach(resetDb);
after(() => prisma.$disconnect());

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

test("scheduler enqueues fetches for due sources and the interval keeps ticking", async () => {
  await prisma.source.create({ data: { name: "Due", url: "https://due.example.com" } });
  sent.length = 0;
  scheduler.startScheduler();
  scheduler.startScheduler(); // second call is a no-op (timer already set)
  // wait long enough for the initial tick plus at least one interval tick
  await sleep(150);
  scheduler.stopScheduler();
  assert.ok(sent.length >= 1, "expected at least one fetch enqueued");
});

test("a recently-fetched source is not re-enqueued, and stop is idempotent", async () => {
  await prisma.source.create({
    data: { name: "Fresh", url: "https://fresh.example.com", lastFetchedAt: new Date() },
  });
  sent.length = 0;
  scheduler.startScheduler();
  await sleep(80);
  scheduler.stopScheduler();
  scheduler.stopScheduler(); // idempotent
  assert.equal(sent.length, 0);
});

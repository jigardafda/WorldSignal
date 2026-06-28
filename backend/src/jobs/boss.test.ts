import "../test-utils/env.js";
import { test, after } from "node:test";
import assert from "node:assert/strict";
import { getBoss, stopBoss } from "./boss.js";
import { QUEUES } from "./queues.js";

after(async () => {
  await stopBoss();
});

test("getBoss starts pg-boss, creates queues, and is cached", async () => {
  const a = await getBoss();
  const b = await getBoss();
  assert.equal(a, b, "getBoss should return the cached instance");

  // queues exist → send succeeds and returns a job id
  const jobId = await a.send(QUEUES.fetchSource, { sourceId: "x" });
  assert.ok(jobId);

  // the registered 'error' handler logs without crashing the process
  assert.doesNotThrow(() => a.emit("error", new Error("synthetic")));
});

test("stopBoss is idempotent", async () => {
  await stopBoss();
  await stopBoss(); // second call no-ops
});

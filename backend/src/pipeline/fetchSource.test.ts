import "../test-utils/env.js";
import { test, beforeEach, after } from "node:test";
import assert from "node:assert/strict";
import { prisma } from "../db/prisma.js";
import { resetDb } from "../test-utils/db.js";
import { fetchSource } from "./fetchSource.js";
import { startTestServer, SAMPLE_RSS, type TestServer } from "../test-utils/httpServer.js";

beforeEach(resetDb);
after(() => prisma.$disconnect());

test("fetches an RSS source and stores new raw items", async () => {
  let server: TestServer | null = null;
  try {
    server = await startTestServer(() => ({ body: SAMPLE_RSS }));
    const src = await prisma.source.create({ data: { name: "Feed", url: server.url } });
    const ids = await fetchSource(src.id);
    assert.equal(ids.length, 2);
    const refreshed = await prisma.source.findUnique({ where: { id: src.id } });
    assert.ok(refreshed?.lastSuccessAt);
    assert.equal(refreshed?.failureCount, 0);

    // second fetch dedupes by (sourceId, guid)
    const again = await fetchSource(src.id);
    assert.equal(again.length, 0);
  } finally {
    await server?.close();
  }
});

test("a failing fetch increments failureCount", async () => {
  const src = await prisma.source.create({
    data: { name: "Bad", url: "http://127.0.0.1:1/does-not-exist" },
  });
  const ids = await fetchSource(src.id);
  assert.deepEqual(ids, []);
  const refreshed = await prisma.source.findUnique({ where: { id: src.id } });
  assert.equal(refreshed?.failureCount, 1);
  assert.ok(refreshed?.lastFailureAt);
});

test("disabled and missing sources are skipped", async () => {
  const src = await prisma.source.create({
    data: { name: "Off", url: "https://off.example.com", enabled: false },
  });
  assert.deepEqual(await fetchSource(src.id), []);
  assert.deepEqual(await fetchSource("missing"), []);
});

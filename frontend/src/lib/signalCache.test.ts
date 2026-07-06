import "fake-indexeddb/auto";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { LiveSignal } from "./api";
import { clearCache, getCached, mergeCached } from "./signalCache";

const DB_NAME = "worldsignal-live";

function resetDB(): Promise<void> {
  return new Promise((resolve) => {
    const req = indexedDB.deleteDatabase(DB_NAME);
    req.onsuccess = () => resolve();
    req.onerror = () => resolve();
    req.onblocked = () => resolve();
  });
}

function sig(id: string, lastSeenAt: string, severity = "LOW"): LiveSignal {
  return { id, title: id, country: "US", region: null, city: null, severity, eventType: "GENERAL", lastSeenAt };
}

// Timestamps must be relative to "now": mergeCached evicts anything older than
// 24h, so hardcoded calendar dates turn into a time-bomb that starts failing a
// day after they were written. `ago(minutes)` keeps records inside the window.
const ago = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString();

beforeEach(resetDB);

describe("signalCache", () => {
  it("returns [] before anything is cached", async () => {
    expect(await getCached(0)).toEqual([]);
  });

  it("round-trips records newest-first and filters by since", async () => {
    const midTs = ago(120);
    await mergeCached([sig("old", ago(180)), sig("mid", midTs), sig("new", ago(60))]);
    const all = await getCached(0);
    expect(all.map((r) => r.id)).toEqual(["new", "mid", "old"]);
    // stored bookkeeping field is stripped on read
    expect((all[0] as unknown as Record<string, unknown>)._ts).toBeUndefined();

    const since = Date.parse(midTs);
    expect((await getCached(since)).map((r) => r.id)).toEqual(["new", "mid"]);
  });

  it("upserts by id rather than duplicating", async () => {
    await mergeCached([sig("a", ago(60), "LOW")]);
    await mergeCached([sig("a", ago(55), "CRITICAL")]);
    const rows = await getCached(0);
    expect(rows).toHaveLength(1);
    expect(rows[0].severity).toBe("CRITICAL");
  });

  it("clearCache empties the store", async () => {
    await mergeCached([sig("a", ago(60))]);
    await clearCache();
    expect(await getCached(0)).toEqual([]);
  });

  it("evicts records older than 24h on merge", async () => {
    const old = new Date(Date.now() - 48 * 60 * 60_000).toISOString();
    const fresh = new Date(Date.now() - 60_000).toISOString();
    await mergeCached([sig("old", old), sig("fresh", fresh)]);
    const rows = await getCached(0);
    expect(rows.map((r) => r.id)).toEqual(["fresh"]);
  });

  it("caps the store at 5000 rows, evicting the oldest", async () => {
    const base = Date.now();
    const batch: LiveSignal[] = [];
    for (let i = 0; i < 5010; i++) {
      // increasing timestamps so ids 0..9 are the oldest and get evicted
      batch.push(sig(`s${i}`, new Date(base - (5010 - i) * 1000).toISOString()));
    }
    await mergeCached(batch);
    const rows = await getCached(0);
    expect(rows).toHaveLength(5000);
    expect(rows.some((r) => r.id === "s0")).toBe(false); // oldest gone
    expect(rows.some((r) => r.id === "s5009")).toBe(true); // newest kept
  });
});

describe("signalCache without IndexedDB", () => {
  const real = globalThis.indexedDB;
  beforeEach(() => {
    // @ts-expect-error simulate an environment with no IndexedDB
    delete globalThis.indexedDB;
  });
  afterEach(() => {
    globalThis.indexedDB = real;
  });

  it("degrades to no-ops", async () => {
    expect(await getCached(0)).toEqual([]);
    await expect(mergeCached([sig("a", "2026-07-05T10:00:00Z")])).resolves.toBeUndefined();
    await expect(clearCache()).resolves.toBeUndefined();
  });
});

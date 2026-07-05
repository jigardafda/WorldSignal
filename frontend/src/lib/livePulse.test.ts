import { describe, expect, it } from "vitest";
import { timeAgo, tickerItems, topMovers, velocity, type PulseRec } from "./livePulse";

const NOW = 1_000_000_000_000;
const ago = (secs: number) => NOW - secs * 1000;

function rec(id: string, category: string, country: string, secsAgo: number): PulseRec {
  return { id, title: id, category, country, lat: 0, lng: 0, lastSeenMs: ago(secsAgo) };
}

describe("velocity", () => {
  it("counts events in the last 60s overall and per category", () => {
    const recs = [
      rec("a", "DISASTER", "US", 10), // in
      rec("b", "DISASTER", "US", 59), // in
      rec("c", "CONFLICT", "FR", 30), // in
      rec("d", "CONFLICT", "FR", 61), // out (>60s)
      rec("e", "POLITICS", "GB", 600), // out
    ];
    const v = velocity(recs, NOW);
    expect(v.perMin).toBe(3);
    expect(v.byCategory).toEqual({ DISASTER: 2, CONFLICT: 1 });
  });
  it("is zero for an empty set", () => {
    expect(velocity([], NOW)).toEqual({ perMin: 0, byCategory: {} });
  });
  it("ignores unknown (0) timestamps", () => {
    expect(velocity([{ ...rec("x", "X", "US", 0), lastSeenMs: 0 }], NOW).perMin).toBe(0);
  });
});

describe("topMovers", () => {
  const windowMs = 60 * 60_000; // 1h ⇒ recent half = last 30m
  it("surfaces categories surging in the recent half, sorted by ratio", () => {
    const recs = [
      // CONFLICT: 1 older, 3 recent ⇒ ratio 3
      rec("c1", "CONFLICT", "FR", 40 * 60), rec("c2", "CONFLICT", "FR", 5 * 60), rec("c3", "CONFLICT", "FR", 6 * 60), rec("c4", "CONFLICT", "FR", 7 * 60),
      // ECONOMY: 2 older, 2 recent ⇒ ratio 1
      rec("e1", "ECONOMY", "US", 45 * 60), rec("e2", "ECONOMY", "US", 50 * 60), rec("e3", "ECONOMY", "US", 2 * 60), rec("e4", "ECONOMY", "US", 3 * 60),
      // DISASTER: 0 older, 2 recent ⇒ "new", ratio = recent = 2
      rec("d1", "DISASTER", "JP", 4 * 60), rec("d2", "DISASTER", "JP", 8 * 60),
    ];
    const movers = topMovers(recs, (r) => r.category, NOW, windowMs, 3);
    expect(movers.map((m) => m.key)).toEqual(["CONFLICT", "DISASTER", "ECONOMY"]);
    expect(movers[0]).toMatchObject({ key: "CONFLICT", recent: 3, older: 1, ratio: 3 });
    expect(movers[1]).toMatchObject({ key: "DISASTER", recent: 2, older: 0, ratio: 2 });
  });
  it("excludes keys with no recent activity and respects the limit", () => {
    const recs = [
      rec("a", "A", "US", 5 * 60), rec("b", "B", "US", 6 * 60), rec("c", "C", "US", 7 * 60), rec("d", "D", "US", 8 * 60),
      rec("old", "OLD", "US", 50 * 60), // only older half ⇒ excluded
    ];
    const movers = topMovers(recs, (r) => r.category, NOW, windowMs, 2);
    expect(movers).toHaveLength(2);
    expect(movers.some((m) => m.key === "OLD")).toBe(false);
  });
  it("ignores events outside the window", () => {
    const recs = [rec("x", "X", "US", 90 * 60)]; // older than the 60m window
    expect(topMovers(recs, (r) => r.category, NOW, windowMs)).toEqual([]);
  });
  it("can key by country", () => {
    const recs = [rec("a", "DISASTER", "JP", 5 * 60), rec("b", "CONFLICT", "JP", 6 * 60)];
    const movers = topMovers(recs, (r) => r.country, NOW, windowMs);
    expect(movers[0]).toMatchObject({ key: "JP", recent: 2 });
  });
});

describe("tickerItems", () => {
  it("returns newest-first and caps the count without mutating input", () => {
    const recs = [rec("old", "A", "US", 300), rec("new", "A", "US", 5), rec("mid", "A", "US", 60)];
    const out = tickerItems(recs, 2);
    expect(out.map((r) => r.id)).toEqual(["new", "mid"]);
    expect(recs[0].id).toBe("old"); // original order untouched
  });
});

describe("timeAgo", () => {
  it("formats seconds, minutes, hours, days", () => {
    expect(timeAgo(ago(5), NOW)).toBe("5s");
    expect(timeAgo(ago(120), NOW)).toBe("2m");
    expect(timeAgo(ago(3 * 3600), NOW)).toBe("3h");
    expect(timeAgo(ago(2 * 86400), NOW)).toBe("2d");
  });
  it("returns a dash for unknown timestamps and never goes negative", () => {
    expect(timeAgo(0, NOW)).toBe("—");
    expect(timeAgo(NaN, NOW)).toBe("—");
    expect(timeAgo(NOW + 5000, NOW)).toBe("0s"); // future clamps to 0
  });
});

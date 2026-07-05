import { describe, expect, it } from "vitest";
import { influenceRank, isBreaking, markerSize, newBreaking, recencyOpacity, ringWidth, sentimentColor, severityRank } from "./liveMarkers";

describe("ringWidth", () => {
  it("is 0 for a single (or unknown) source and grows, capped at 6", () => {
    expect(ringWidth(1)).toBe(0);
    expect(ringWidth(0)).toBe(0);
    expect(ringWidth(null)).toBe(0);
    expect(ringWidth(undefined)).toBe(0);
    expect(ringWidth(2)).toBe(1);
    expect(ringWidth(5)).toBe(4);
    expect(ringWidth(50)).toBe(6); // capped
  });
});

describe("influenceRank", () => {
  it("ranks influence and defaults null/unknown to 0", () => {
    expect(influenceRank("LOW")).toBe(1);
    expect(influenceRank("MEDIUM")).toBe(2);
    expect(influenceRank("HIGH")).toBe(3);
    expect(influenceRank("CRITICAL")).toBe(4);
    expect(influenceRank("NEGLIGIBLE")).toBe(0);
    expect(influenceRank(null)).toBe(0);
    expect(influenceRank("WAT")).toBe(0);
  });
});

describe("sentimentColor", () => {
  it("maps sentiment codes to hex and falls back to gray", () => {
    expect(sentimentColor("POSITIVE")).toBe("#2f9e44");
    expect(sentimentColor("NEGATIVE")).toBe("#e03131");
    expect(sentimentColor("NEUTRAL")).toBe("#868e96");
    expect(sentimentColor("MIXED")).toBe("#f08c00");
    expect(sentimentColor(null)).toBe("#868e96");
    expect(sentimentColor("WAT")).toBe("#868e96");
  });
});

describe("severityRank", () => {
  it("ranks the four levels and defaults unknown/empty to 0", () => {
    expect(severityRank("LOW")).toBe(0);
    expect(severityRank("MEDIUM")).toBe(1);
    expect(severityRank("HIGH")).toBe(2);
    expect(severityRank("CRITICAL")).toBe(3);
    expect(severityRank("WAT")).toBe(0);
    expect(severityRank(null)).toBe(0);
    expect(severityRank(undefined)).toBe(0);
  });
});

describe("markerSize", () => {
  it("scales from 10px (LOW) to 22px (CRITICAL)", () => {
    expect(markerSize("LOW")).toBe(10);
    expect(markerSize("MEDIUM")).toBe(14);
    expect(markerSize("HIGH")).toBe(18);
    expect(markerSize("CRITICAL")).toBe(22);
    expect(markerSize(null)).toBe(10);
  });
});

describe("isBreaking", () => {
  it("is true only for HIGH and CRITICAL", () => {
    expect(isBreaking("CRITICAL")).toBe(true);
    expect(isBreaking("HIGH")).toBe(true);
    expect(isBreaking("MEDIUM")).toBe(false);
    expect(isBreaking("LOW")).toBe(false);
    expect(isBreaking(null)).toBe(false);
  });
});

describe("recencyOpacity", () => {
  const now = Date.parse("2026-07-05T12:00:00Z");
  const windowMs = 60 * 60_000; // 1h

  it("is 1.0 for a just-seen event", () => {
    expect(recencyOpacity("2026-07-05T12:00:00Z", now, windowMs)).toBe(1);
  });
  it("floors at 0.35 at/after the window edge", () => {
    expect(recencyOpacity("2026-07-05T11:00:00Z", now, windowMs)).toBe(0.35); // exactly 1h old
    expect(recencyOpacity("2026-07-05T09:00:00Z", now, windowMs)).toBe(0.35); // older than window
  });
  it("interpolates linearly in between", () => {
    // 30 min old over a 60 min window ⇒ halfway between 1.0 and 0.35 = 0.675
    expect(recencyOpacity("2026-07-05T11:30:00Z", now, windowMs)).toBeCloseTo(0.675, 5);
  });
  it("returns 1 for missing/invalid timestamps, non-positive windows, and future events", () => {
    expect(recencyOpacity(null, now, windowMs)).toBe(1);
    expect(recencyOpacity("not-a-date", now, windowMs)).toBe(1);
    expect(recencyOpacity("2026-07-05T12:00:00Z", now, 0)).toBe(1);
    expect(recencyOpacity("2026-07-05T13:00:00Z", now, windowMs)).toBe(1); // future
  });
});

describe("newBreaking", () => {
  const recs = [
    { id: "a", severity: "CRITICAL", lastSeenAt: null },
    { id: "b", severity: "LOW", lastSeenAt: null },
    { id: "c", severity: "HIGH", lastSeenAt: null },
  ];

  it("returns breaking records not seen in the previous poll", () => {
    const out = newBreaking(recs, new Set(["a"]), false);
    expect(out.map((r) => r.id)).toEqual(["c"]); // a already seen, b not breaking
  });
  it("suppresses everything on the first poll", () => {
    expect(newBreaking(recs, new Set(), true)).toEqual([]);
  });
  it("returns all new breaking records when none were seen before", () => {
    expect(newBreaking(recs, new Set(), false).map((r) => r.id)).toEqual(["a", "c"]);
  });
});

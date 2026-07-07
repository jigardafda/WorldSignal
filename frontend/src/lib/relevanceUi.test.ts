import { describe, expect, it } from "vitest";
import { formatAge, humanizeReason, rankItems, scoreBand, scorePct } from "./relevanceUi";
import type { FeedItem } from "./api";

describe("relevanceUi", () => {
  it("bands scores from strong to background", () => {
    expect(scoreBand(9).label).toBe("Strong match");
    expect(scoreBand(5).label).toBe("Relevant");
    expect(scoreBand(2.5).label).toBe("Loosely related");
    expect(scoreBand(0.5).label).toBe("Background");
  });

  it("clamps the ring percentage to 0–100", () => {
    expect(scorePct(9.4)).toBe(94);
    expect(scorePct(-1)).toBe(0);
    expect(scorePct(20)).toBe(100);
  });

  it("humanizes interest keys with the right kind", () => {
    expect(humanizeReason("entity:Marcus Vale")).toEqual({ label: "Marcus Vale", kind: "entity" });
    expect(humanizeReason("keyword:matcha")).toEqual({ label: "“matcha”", kind: "keyword" });
    expect(humanizeReason("country:US")).toEqual({ label: "US", kind: "place" });
    expect(humanizeReason("sentiment:NEGATIVE").kind).toBe("sentiment");
    // tag maps through the category label of its domain.
    expect(humanizeReason("tag:DISASTER.EARTHQUAKE").kind).toBe("topic");
  });

  it("formats age compactly", () => {
    expect(formatAge(0.25)).toBe("15m ago");
    expect(formatAge(3)).toBe("3h ago");
    expect(formatAge(50)).toBe("2d ago");
  });

  it("ranks by relevance or recency", () => {
    const items = [
      { id: "a", score: 5, ageHours: 10 },
      { id: "b", score: 9, ageHours: 40 },
      { id: "c", score: 7, ageHours: 1 },
    ] as FeedItem[];
    expect(rankItems(items, "relevance").map((i) => i.id)).toEqual(["b", "c", "a"]);
    expect(rankItems(items, "recency").map((i) => i.id)).toEqual(["c", "a", "b"]);
  });
});

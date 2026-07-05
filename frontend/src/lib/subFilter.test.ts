import { describe, expect, it } from "vitest";
import { cleanFilter, conditionCount, filterSummary } from "./subFilter";

describe("cleanFilter", () => {
  it("drops empty arrays, blank strings, zero/default thresholds", () => {
    expect(cleanFilter({
      tags: [], countries: ["US", ""], regions: [], sentiment: [],
      entities: ["", "  "], keyword: "  ", minConfidence: 0, minRelevance: 0,
      minSeverity: "LOW", minInfluence: "LOW",
    })).toEqual({ countries: ["US"] });
  });

  it("keeps set fields and trims", () => {
    expect(cleanFilter({
      tags: ["DISASTER"], keyword: "  quake ", minConfidence: 0.5,
      minSeverity: "HIGH", minInfluence: "MEDIUM", sentiment: ["NEGATIVE"],
    })).toEqual({
      tags: ["DISASTER"], keyword: "quake", minConfidence: 0.5,
      minSeverity: "HIGH", minInfluence: "MEDIUM", sentiment: ["NEGATIVE"],
    });
  });

  it("empty filter stays empty (match everything)", () => {
    expect(cleanFilter({})).toEqual({});
  });
});

describe("conditionCount / filterSummary", () => {
  it("counts constrained dimensions", () => {
    expect(conditionCount({})).toBe(0);
    expect(conditionCount({ tags: ["X"], minSeverity: "HIGH" })).toBe(2);
    expect(conditionCount({ minSeverity: "LOW" })).toBe(0); // default → not a condition
  });
  it("summarizes", () => {
    expect(filterSummary({})).toBe("All signals");
    expect(filterSummary({ countries: ["US"] })).toBe("1 condition");
    expect(filterSummary({ countries: ["US"], keyword: "x" })).toBe("2 conditions");
  });
});

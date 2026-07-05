import { describe, expect, it } from "vitest";
import { aggregateByCountry, fillFor, legendFor, lerpHex, metricMax, metricValue, type CountryAgg } from "./choropleth";

const marker = (country: string, severity: string, sentiment: string | null) => ({ country, severity, sentiment });

describe("aggregateByCountry", () => {
  it("tallies count, high-severity, and sentiment per country; skips no-country", () => {
    const m = aggregateByCountry([
      marker("US", "CRITICAL", "NEGATIVE"),
      marker("US", "LOW", "POSITIVE"),
      marker("US", "HIGH", "NEUTRAL"),
      marker("FR", "MEDIUM", "POSITIVE"),
      { country: null, severity: "HIGH", sentiment: "NEGATIVE" },
    ]);
    expect(m.get("US")).toEqual({ count: 3, hi: 2, pos: 1, neg: 1 });
    expect(m.get("FR")).toEqual({ count: 1, hi: 0, pos: 1, neg: 0 });
    expect(m.has("__null__")).toBe(false);
  });
});

describe("metricValue", () => {
  const a: CountryAgg = { count: 4, hi: 1, pos: 3, neg: 1 };
  it("returns count, high-share, and net sentiment", () => {
    expect(metricValue(a, "count")).toBe(4);
    expect(metricValue(a, "severity")).toBe(0.25); // 1/4
    expect(metricValue(a, "sentiment")).toBe(0.5); // (3-1)/4
  });
  it("is 0 for an empty country on ratio metrics", () => {
    const z: CountryAgg = { count: 0, hi: 0, pos: 0, neg: 0 };
    expect(metricValue(z, "severity")).toBe(0);
    expect(metricValue(z, "sentiment")).toBe(0);
  });
});

describe("metricMax", () => {
  const aggs: CountryAgg[] = [{ count: 2, hi: 0, pos: 0, neg: 0 }, { count: 9, hi: 0, pos: 0, neg: 0 }];
  it("is the largest count for count, and 1 for the fixed-domain metrics", () => {
    expect(metricMax(aggs, "count")).toBe(9);
    expect(metricMax(aggs, "severity")).toBe(1);
    expect(metricMax(aggs, "sentiment")).toBe(1);
    expect(metricMax([], "count")).toBe(1); // never 0
  });
});

describe("lerpHex", () => {
  it("interpolates and clamps t", () => {
    expect(lerpHex("#000000", "#ffffff", 0)).toBe("#000000");
    expect(lerpHex("#000000", "#ffffff", 1)).toBe("#ffffff");
    expect(lerpHex("#000000", "#ffffff", 0.5)).toBe("#808080");
    expect(lerpHex("#000000", "#ffffff", 2)).toBe("#ffffff"); // clamped
  });
});

describe("fillFor", () => {
  it("sequential count: brighter with more, floored so present ≠ invisible", () => {
    const low = fillFor(1, "count", 10);
    const high = fillFor(10, "count", 10);
    expect(low).not.toBe("#e7f0ff"); // floored above the light end
    expect(high).toBe("#1c4ea8"); // full at the max
  });
  it("severity uses the share directly (0..1)", () => {
    expect(fillFor(1, "severity", 1)).toBe("#c92a2a"); // 100% high ⇒ dark red
  });
  it("sentiment diverges around a neutral gray midpoint", () => {
    expect(fillFor(0, "sentiment", 1)).toBe("#ced4da"); // neutral
    expect(fillFor(1, "sentiment", 1)).toBe("#2b8a3e"); // fully positive ⇒ green
    expect(fillFor(-1, "sentiment", 1)).toBe("#c92a2a"); // fully negative ⇒ red
  });
});

describe("legendFor", () => {
  it("gives diverging stops for sentiment and sequential stops otherwise", () => {
    expect(legendFor("sentiment", 1).stops).toHaveLength(3);
    expect(legendFor("count", 42)).toMatchObject({ min: "1", max: "42" });
    expect(legendFor("severity", 1).stops).toHaveLength(2);
  });
});

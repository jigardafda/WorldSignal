import { describe, expect, it } from "vitest";
import { pointSize, toArcs, toPoints, toRings, type PointInput } from "./globeData";

const base = (id: string, over: Partial<PointInput> = {}): PointInput => ({ id, lat: 10, lng: 20, title: id, color: "#0ca678", ...over });

describe("pointSize", () => {
  it("scales with severity", () => {
    expect(pointSize("LOW")).toBeCloseTo(0.18, 5);
    expect(pointSize("CRITICAL")).toBeCloseTo(0.48, 5);
  });
});

describe("toPoints", () => {
  it("maps markers to points with category color and severity size", () => {
    const [p] = toPoints([base("a", { severity: "HIGH", opacity: 0.6 })]);
    expect(p).toMatchObject({ id: "a", lat: 10, lng: 20, color: "#0ca678", opacity: 0.6 });
    expect(p.size).toBeCloseTo(0.38, 5); // HIGH
  });
  it("colors by sentiment when the tint layer is on", () => {
    const [p] = toPoints([base("a", { sentiment: "NEGATIVE" })], true);
    expect(p.color).toBe("#e03131");
  });
});

describe("toArcs", () => {
  const recs = [
    base("old", { lastSeenMs: 100, lat: 1, lng: 1 }),
    base("mid", { lastSeenMs: 200, lat: 2, lng: 2 }),
    base("new", { lastSeenMs: 300, lat: 3, lng: 3 }),
    base("notime", { lastSeenMs: 0, lat: 9, lng: 9 }),
  ];
  it("connects the most recent events newest→older, excluding untimed ones", () => {
    const arcs = toArcs(recs);
    expect(arcs).toHaveLength(2); // new→mid, mid→old
    expect(arcs[0]).toMatchObject({ startLat: 3, startLng: 3, endLat: 2, endLng: 2 });
    expect(arcs[1]).toMatchObject({ startLat: 2, endLat: 1 });
  });
  it("caps at maxArcs", () => {
    const many = Array.from({ length: 30 }, (_, i) => base(`s${i}`, { lastSeenMs: i + 1 }));
    expect(toArcs(many, 10)).toHaveLength(9); // 10 recent ⇒ 9 arcs
  });
  it("returns none for fewer than two timed events", () => {
    expect(toArcs([base("a", { lastSeenMs: 1 })])).toEqual([]);
    expect(toArcs([])).toEqual([]);
  });
});

describe("toRings", () => {
  it("rings only breaking + new events", () => {
    const rings = toRings([
      base("a", { breaking: true, isNew: true }),
      base("b", { breaking: true, isNew: false }),
      base("c", { breaking: false, isNew: true }),
    ]);
    expect(rings).toHaveLength(1);
    expect(rings[0]).toMatchObject({ lat: 10, lng: 20 });
  });
});

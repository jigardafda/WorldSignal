import { describe, expect, it } from "vitest";
import { frameMarkers, type FrameMarker } from "./replay";

const WINDOW = 60 * 60_000; // 1h
const END = 1_000_000_000_000; // playhead "now" reference
const at = (minsAgo: number): number => END - minsAgo * 60_000;

type Rec = FrameMarker & { id: string };
const rec = (id: string, minsAgo: number): Rec => ({ id, lastSeenMs: at(minsAgo) });

describe("frameMarkers", () => {
  const recs: Rec[] = [rec("a", 50), rec("b", 30), rec("c", 5), rec("future", -10)];

  it("shows only events that have occurred by the playhead", () => {
    // playhead at 20 min ago ⇒ a (50) and b (30) visible; c (5) not yet; future excluded
    const out = frameMarkers(recs, at(20), WINDOW, null);
    expect(out.map((r) => r.id).sort()).toEqual(["a", "b"]);
  });

  it("fades visible events by age relative to the playhead", () => {
    const out = frameMarkers([rec("edge", 60), rec("fresh", 0)], END, WINDOW, null);
    const byId = Object.fromEntries(out.map((r) => [r.id, r.opacity]));
    expect(byId["fresh"]).toBe(1); // right at the playhead ⇒ full opacity
    expect(byId["edge"]).toBeCloseTo(0.35, 5); // a full window old ⇒ floor
  });

  it("marks events that crossed the playhead since the previous frame as new", () => {
    // advance from 35 min ago to 20 min ago: b (30) crossed; a (50) already shown
    const out = frameMarkers(recs, at(20), WINDOW, at(35));
    const byId = Object.fromEntries(out.map((r) => [r.id, r.isNew]));
    expect(byId["b"]).toBe(true);
    expect(byId["a"]).toBe(false);
  });

  it("never ripples when prevPlayheadMs is null (a seek / first frame)", () => {
    const out = frameMarkers(recs, END, WINDOW, null);
    expect(out.every((r) => r.isNew === false)).toBe(true);
  });

  it("excludes events without a usable timestamp", () => {
    const out = frameMarkers([{ id: "x", lastSeenMs: 0 } as Rec, rec("ok", 1)], END, WINDOW, null);
    expect(out.map((r) => r.id)).toEqual(["ok"]);
  });

  it("preserves other marker fields", () => {
    const src = { id: "a", lastSeenMs: at(1), color: "#e03131", severity: "HIGH", lat: 5, lng: 6 };
    const [out] = frameMarkers([src], END, WINDOW, null);
    expect(out).toMatchObject({ id: "a", color: "#e03131", severity: "HIGH", lat: 5, lng: 6 });
  });
});

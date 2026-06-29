import { describe, expect, it } from "vitest";
import { clampLat, jitter } from "./geo";

describe("clampLat", () => {
  it("clamps to web-mercator range", () => {
    expect(clampLat(95)).toBe(85);
    expect(clampLat(-95)).toBe(-85);
    expect(clampLat(40)).toBe(40);
  });
});

describe("jitter", () => {
  it("is deterministic for a seed and stays near the source", () => {
    const a = jitter(48.85, 2.35, "sig-1");
    const b = jitter(48.85, 2.35, "sig-1");
    expect(a).toEqual(b);
    expect(Math.abs(a[0] - 48.85)).toBeLessThanOrEqual(0.7);
    expect(Math.abs(a[1] - 2.35)).toBeLessThanOrEqual(0.7);
  });
  it("scatters different seeds to different points", () => {
    expect(jitter(0, 0, "a")).not.toEqual(jitter(0, 0, "b"));
  });
  it("keeps latitude on-map even near the poles", () => {
    const [lat] = jitter(85, 10, "x");
    expect(lat).toBeLessThanOrEqual(85);
    expect(lat).toBeGreaterThanOrEqual(-85);
  });
});

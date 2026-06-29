import { afterEach, beforeAll, describe, expect, it } from "vitest";
import { _resetGeoCache, geocode, preloadGeo } from "./geocode";

beforeAll(async () => {
  await preloadGeo();
});
afterEach(() => _resetGeoCache());

describe("geocode", () => {
  it("resolves a city to its coordinates (most precise)", () => {
    const hit = geocode("IN", "Maharashtra", "Mumbai");
    expect(hit?.precision).toBe("city");
    expect(hit!.lat).toBeCloseTo(19.07, 0);
    expect(hit!.lng).toBeCloseTo(72.88, 0);
  });
  it("falls back to the state when no city is given", () => {
    const hit = geocode("IN", "Maharashtra", null);
    expect(hit?.precision).toBe("region");
    expect(hit!.lat).toBeCloseTo(19.75, 0);
    expect(hit!.lat).toBeLessThan(25); // not Delhi (~28.6)
  });
  it("returns null when only the country is known (caller uses the capital)", () => {
    expect(geocode("IN", null, null)).toBeNull();
  });
  it("returns null for an unknown country", () => {
    expect(geocode("ZZ", "Nowhere", "Nowhere")).toBeNull();
  });
  it("returns null without a country", () => {
    expect(geocode(null, "Maharashtra", "Mumbai")).toBeNull();
  });
});

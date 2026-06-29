import { afterEach, describe, expect, it } from "vitest";
import { _resetBoundaryCache, countryOutline } from "./boundaries";

afterEach(() => _resetBoundaryCache());

describe("countryOutline", () => {
  it("returns boundary geometry for a known country (US -> ISO 840)", async () => {
    const us = await countryOutline("US");
    expect(us).not.toBeNull();
    expect(String(us!.id)).toBe("840");
    expect(us!.geometry.type).toMatch(/Polygon/);
  });
  it("returns null for an unknown code", async () => {
    expect(await countryOutline("ZZ")).toBeNull();
  });
});

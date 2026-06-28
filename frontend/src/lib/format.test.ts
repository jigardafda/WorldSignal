import { describe, expect, it } from "vitest";
import { fmtDate, pct, timeAgo } from "./format";

describe("fmtDate", () => {
  it("formats valid dates and dashes empties/invalids", () => {
    expect(fmtDate(null)).toBe("—");
    expect(fmtDate("")).toBe("—");
    expect(fmtDate("not-a-date")).toBe("—");
    expect(fmtDate("2026-01-02T03:04:05.000Z")).not.toBe("—");
  });
});

describe("pct", () => {
  it("renders percentages and dashes null", () => {
    expect(pct(null)).toBe("—");
    expect(pct(0.826)).toBe("83%");
    expect(pct(1)).toBe("100%");
  });
});

describe("timeAgo", () => {
  it("handles empty + invalid", () => {
    expect(timeAgo(null)).toBe("never");
    expect(timeAgo("nope")).toBe("never");
  });
  it("renders relative units", () => {
    const now = Date.now();
    expect(timeAgo(new Date(now - 5000).toISOString())).toMatch(/s ago/);
    expect(timeAgo(new Date(now - 5 * 60000).toISOString())).toMatch(/m ago/);
    expect(timeAgo(new Date(now - 5 * 3600000).toISOString())).toMatch(/h ago/);
    expect(timeAgo(new Date(now - 5 * 86400000).toISOString())).toMatch(/d ago/);
  });
});

import { describe, expect, it } from "vitest";
import { fmtDate, fmtDay, languageName, pct, timeAgo } from "./format";

describe("languageName", () => {
  it("maps known codes, falls back to upper-case, handles empty", () => {
    expect(languageName("fr")).toBe("French");
    expect(languageName("EN")).toBe("English");
    expect(languageName("xx")).toBe("XX");
    expect(languageName(null)).toBe("");
    expect(languageName(undefined)).toBe("");
  });
});

describe("fmtDate", () => {
  it("formats valid dates and dashes empties/invalids", () => {
    expect(fmtDate(null)).toBe("—");
    expect(fmtDate("")).toBe("—");
    expect(fmtDate("not-a-date")).toBe("—");
    expect(fmtDate("2026-01-02T03:04:05.000Z")).not.toBe("—");
  });
});

describe("fmtDay", () => {
  it("returns a dash for empty/invalid input", () => {
    expect(fmtDay(null)).toBe("—");
    expect(fmtDay("")).toBe("—");
    expect(fmtDay("not-a-date")).toBe("—");
  });
  it("formats as an ordinal day, month name, and year (no time)", () => {
    // Use a midday UTC time so the local date is stable across timezones.
    expect(fmtDay("2026-05-05T12:00:00Z")).toBe("5th May 2026");
    expect(fmtDay("2026-05-01T12:00:00Z")).toBe("1st May 2026");
    expect(fmtDay("2026-05-02T12:00:00Z")).toBe("2nd May 2026");
    expect(fmtDay("2026-05-03T12:00:00Z")).toBe("3rd May 2026");
    expect(fmtDay("2026-05-11T12:00:00Z")).toBe("11th May 2026");
    expect(fmtDay("2026-05-21T12:00:00Z")).toBe("21st May 2026");
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

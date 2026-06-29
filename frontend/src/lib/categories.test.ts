import { describe, expect, it } from "vitest";
import { CATEGORIES, categoryColor, categoryLabel, domainOf } from "./categories";

describe("categories", () => {
  it("derives the taxonomy domain from an eventType", () => {
    expect(domainOf("DISASTER.EARTHQUAKE")).toBe("DISASTER");
    expect(domainOf("POLITICS.ELECTIONS")).toBe("POLITICS");
    expect(domainOf("WEIRD.THING")).toBe("GENERAL"); // unknown domain
    expect(domainOf(null)).toBe("GENERAL");
    expect(domainOf("")).toBe("GENERAL");
  });
  it("maps codes to distinct colors and labels", () => {
    expect(categoryColor("DISASTER")).toBe("#e03131");
    expect(categoryLabel("PUBLIC_HEALTH")).toBe("Public Health");
    expect(categoryColor("NOPE")).toBe("#868e96"); // fallback
    expect(categoryLabel("NOPE")).toBe("NOPE");
    const colors = new Set(CATEGORIES.map((c) => c.color));
    expect(colors.size).toBe(CATEGORIES.length); // all distinct
  });
});

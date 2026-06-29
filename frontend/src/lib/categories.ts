/** Top-level taxonomy domains used as map layers, each with a distinct color.
 * Mirrors the domains in the backend taxonomy (POLITICS, ECONOMY, …). */
export interface Category {
  code: string;
  label: string;
  color: string;
}

export const CATEGORIES: Category[] = [
  { code: "POLITICS", label: "Politics", color: "#4263eb" },
  { code: "ECONOMY", label: "Economy", color: "#0ca678" },
  { code: "BUSINESS", label: "Business", color: "#1098ad" },
  { code: "TECHNOLOGY", label: "Technology", color: "#2f6df6" },
  { code: "DISASTER", label: "Disaster", color: "#e03131" },
  { code: "PUBLIC_HEALTH", label: "Public Health", color: "#2f9e44" },
  { code: "LEGAL", label: "Legal", color: "#7048e8" },
  { code: "CONFLICT", label: "Conflict", color: "#e8590c" },
  { code: "SPORTS", label: "Sports", color: "#66a80f" },
  { code: "GENERAL", label: "General", color: "#868e96" },
];

const BY_CODE = new Map(CATEGORIES.map((c) => [c.code, c]));

/** The taxonomy domain (layer) for an eventType code like "DISASTER.EARTHQUAKE".
 * Unknown or missing codes fall back to GENERAL. */
export function domainOf(eventType: string | null | undefined): string {
  if (!eventType) return "GENERAL";
  const d = eventType.split(".")[0];
  return BY_CODE.has(d) ? d : "GENERAL";
}

export function categoryColor(code: string): string {
  return BY_CODE.get(code)?.color ?? "#868e96";
}

export function categoryLabel(code: string): string {
  return BY_CODE.get(code)?.label ?? code;
}

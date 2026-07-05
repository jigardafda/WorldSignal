// The subscription filter model shared by the visual builder and mirrored by the
// backend matcher. A zero/empty value matches everything; each set field narrows
// the match (AND). Kept separate from UI so the shape + cleanup are unit-tested.

export interface SubFilter {
  tags?: string[]; // taxonomy category/subcategory codes (hierarchical)
  countries?: string[]; // ISO alpha-2
  regions?: string[]; // region/state names
  sentiment?: string[]; // POSITIVE | NEUTRAL | NEGATIVE
  entities?: string[]; // entity names
  keyword?: string; // substring of title/summary
  minConfidence?: number; // 0..1
  minRelevance?: number; // 0..1
  minSeverity?: string; // LOW | MEDIUM | HIGH | CRITICAL
  minInfluence?: string; // LOW | MEDIUM | HIGH
}

export const SEVERITIES = ["LOW", "MEDIUM", "HIGH", "CRITICAL"];
export const INFLUENCES = ["LOW", "MEDIUM", "HIGH"];
export const SENTIMENTS = ["POSITIVE", "NEUTRAL", "NEGATIVE"];

/** cleanFilter drops empty fields so the stored filter and generated code stay
 * minimal (an empty object means "match everything"). */
export function cleanFilter(f: SubFilter): SubFilter {
  const out: SubFilter = {};
  const arr = (v?: string[]) => v && v.filter((x) => x.trim() !== "");
  if (arr(f.tags)?.length) out.tags = arr(f.tags);
  if (arr(f.countries)?.length) out.countries = arr(f.countries);
  if (arr(f.regions)?.length) out.regions = arr(f.regions);
  if (arr(f.sentiment)?.length) out.sentiment = arr(f.sentiment);
  if (arr(f.entities)?.length) out.entities = arr(f.entities);
  if (f.keyword && f.keyword.trim()) out.keyword = f.keyword.trim();
  if (typeof f.minConfidence === "number" && f.minConfidence > 0) out.minConfidence = f.minConfidence;
  if (typeof f.minRelevance === "number" && f.minRelevance > 0) out.minRelevance = f.minRelevance;
  if (f.minSeverity && f.minSeverity !== "LOW") out.minSeverity = f.minSeverity;
  if (f.minInfluence && f.minInfluence !== "LOW") out.minInfluence = f.minInfluence;
  return out;
}

/** conditionCount is how many dimensions the filter constrains (0 = match all). */
export function conditionCount(f: SubFilter): number {
  return Object.keys(cleanFilter(f)).length;
}

/** filterSummary is a short human description for a table cell / header. */
export function filterSummary(f: SubFilter): string {
  const n = conditionCount(f);
  return n === 0 ? "All signals" : `${n} condition${n === 1 ? "" : "s"}`;
}
